/*

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	"context"
	"errors"
	"fmt"
	"github.com/go-logr/logr"
	"github.com/robfig/cron"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/record"
	pvcv1 "pvc-operator/api/v1"
	"pvc-operator/utils"
	"reflect"
	runtime1 "runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sync"
)

// CronPersistentVolumeAutoscalerReconciler reconciles a CronPersistentVolumeAutoscaler object
type CronPersistentVolumeAutoscalerReconciler struct {
	client.Client
	Log              logr.Logger
	Scheme           *runtime.Scheme
	Jobs             sync.Map
	Recorder         record.EventRecorder
	K8sClient        kubernetes.Interface
	K8sDynamicClient dynamic.Interface
	CpEnvs           map[string]string
}

// +kubebuilder:rbac:groups=ipaas.icks.inspur.com,resources=cronpersistentvolumeautoscalers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=ipaas.icks.inspur.com,resources=cronpersistentvolumeautoscalers/status,verbs=get;update;patch

func (r *CronPersistentVolumeAutoscalerReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	utils.Logger.Info("-------------start cp reconcile--------------")
	defer func() {
		if err := recover(); err != nil {
			const size = 64 << 10
			buf := make([]byte, size)
			buf = buf[:runtime1.Stack(buf, false)]
			utils.Logger.Error(fmt.Sprintf("cp reconcile error: %v", err))
		}
	}()

	log := r.Log.WithValues("cronpersistentvolumeautoscaler", req.NamespacedName)
	cp := &pvcv1.CronPersistentVolumeAutoscaler{}

	err := r.Get(ctx, req.NamespacedName, cp)
	if err != nil {
		log.Info("unable to fetch cp")
		return ctrl.Result{}, err
	}

	//新建cp时，设置status
	if cp.Status.Conditions == nil {
		return ctrl.Result{}, r.setCpStatus(ctx, log, cp)
	}

	//检查cp是否准备就绪，更新status，false--》true
	if cp.Status.Phase == "False" {
		return r.processCreating(ctx, log, cp)
	}

	//根据pva配置，生成任务；cp删除
	if cp.Status.Phase == "True" {
		return r.processUpdating(ctx, log, cp)
	}

	return ctrl.Result{}, nil
}

func (r *CronPersistentVolumeAutoscalerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&pvcv1.CronPersistentVolumeAutoscaler{}).
		Complete(r)
}

/*
*
设置cronpva状态
*/
func (r *CronPersistentVolumeAutoscalerReconciler) setCpStatus(ctx context.Context, log logr.Logger, cp *pvcv1.CronPersistentVolumeAutoscaler) error {
	conditions := []pvcv1.Condition{}
	condition := pvcv1.Condition{}
	condition.Type = "RecommendationProvided"
	condition.Status = "False"
	condition.Reason = "TargetPvcStatusUnknown"
	condition.Message = "No check target pva is availability."
	condition.LastTransitionTime = metav1.Now()
	conditions = append(conditions, condition)
	cp.Status.Conditions = conditions
	cp.Status.Phase = "False"
	if err := r.Status().Update(ctx, cp); err != nil {
		log.Error(err, "unable to update cp status")
		return err
	}
	return nil
}

/*
创建cronpva
*/
func (r *CronPersistentVolumeAutoscalerReconciler) processCreating(ctx context.Context, log logr.Logger, cp *pvcv1.CronPersistentVolumeAutoscaler) (ctrl.Result, error) {
	var eventType, eventReason, eventMessage string

	//校验cp是否开启
	if cp.Spec.Enable != true {
		err := errors.New("cp " + cp.Name + " is not enable")
		log.Error(err, "cp "+cp.Name+" is not enable")
		eventType = corev1.EventTypeWarning
		eventReason = "FailedCreateCronPva"
		eventMessage = "enable is not true"
		r.pvaRecordEvent(cp, eventType, eventReason, eventMessage)
		return ctrl.Result{}, err
	}

	//校验目标pva信息
	if utils.IsEmpty(cp.Spec.TargetRef) || utils.IsEmpty(cp.Spec.TargetRef.PvaName) {
		err := errors.New("cp " + cp.Name + " is config empty")
		log.Error(err, "cp "+cp.Name+" is config empty")
		eventType = corev1.EventTypeWarning
		eventReason = "FailedCreateCronPva"
		eventMessage = "not config target pva"
		r.pvaRecordEvent(cp, eventType, eventReason, eventMessage)
		return ctrl.Result{}, err
	}

	targetPva, err := r.K8sDynamicClient.Resource(schema.GroupVersionResource{Group: "ipaas.icks.inspur.com", Version: "v1", Resource: "persistentvolumeautoscalers"}).Namespace(cp.GetNamespace()).Get(context.TODO(), cp.Spec.TargetRef.PvaName, metav1.GetOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		err := errors.New("cp " + cp.Name + " target pva " + cp.Spec.TargetRef.PvaName + " error")
		log.Error(err, "cp "+cp.Name+" target pva "+cp.Spec.TargetRef.PvaName+" error")
		return ctrl.Result{}, err
	}

	if apierrors.IsNotFound(err) || targetPva == nil {
		err := errors.New("cp " + cp.Name + " target pva is nil")
		log.Error(err, "cp "+cp.Name+" target pva is nil")
		eventType = corev1.EventTypeWarning
		eventReason = "FailedCheckTargetPva"
		eventMessage = "not found target pvc " + cp.GetName()
		r.pvaRecordEvent(cp, eventType, eventReason, eventMessage)
		return ctrl.Result{}, err
	}

	//校验targetpva状态
	targetPvaStatus := targetPva.UnstructuredContent()["status"]
	targetPvaPhase := targetPvaStatus.(map[string]interface{})["phase"].(string)
	if targetPvaPhase != "True" {
		err := errors.New("cp " + cp.Name + " target pva status is not true")
		log.Error(err, "cp "+cp.Name+" target pva status is not true")
		eventType = corev1.EventTypeWarning
		eventReason = "FailedCheckTargetPva"
		eventMessage = "target pva status is not true"
		r.pvaRecordEvent(cp, eventType, eventReason, eventMessage)
		return ctrl.Result{}, err
	}

	//校验condition类型
	spec := targetPva.UnstructuredContent()["spec"]
	conditions := spec.(map[string]interface{})["conditions"]
	condition0 := conditions.([]interface{})[0].(map[string]interface{})
	condition0Type := condition0["type"].(string)
	if condition0Type != "volume-cron" {
		err := errors.New("cp " + cp.Name + " target pva condition type config error")
		log.Error(err, "cp "+cp.Name+" target pva condition type config error")
		return ctrl.Result{}, err
	}

	//更新cp状态
	if !reflect.DeepEqual("True", cp.Status.Phase) {
		condition := pvcv1.Condition{}
		condition.Type = "RecommendationProvided"
		condition.Status = "True"
		condition.Reason = "TargetPvcStatusAvailable"
		condition.Message = "check target pva is availability."
		condition.LastTransitionTime = metav1.Now()
		cp.Status.Conditions = append(cp.Status.Conditions, condition)
		cp.Status.Phase = "True"
		if err := r.Status().Update(ctx, cp); err != nil {
			log.Error(err, "update cp status error")
			return ctrl.Result{}, err
		}
	}
	eventType = corev1.EventTypeNormal
	eventReason = "SuccessfulCreate"
	eventMessage = "cp " + cp.Name + " is created"
	r.pvaRecordEvent(cp, eventType, eventReason, eventMessage)
	return ctrl.Result{}, nil
}

/*
更新cronpva
*/
func (r *CronPersistentVolumeAutoscalerReconciler) processUpdating(ctx context.Context, log logr.Logger, cp *pvcv1.CronPersistentVolumeAutoscaler) (ctrl.Result, error) {
	//获取注解信息
	annotations := cp.GetAnnotations()

	//create事件
	if annotations == nil && len(annotations) < 1 {
		//校验
		result, err := r.checkCreateOrUpdate(ctx, log, cp)
		if err != nil {
			return result, err
		}

		return r.cpJobCreateOrUpdate(ctx, log, cp)
	}

	delF, delFExists := annotations["delete-flag"]
	chgF, chgFExists := annotations["change-flag"]
	if delFExists == true && delF == "true" && chgFExists == true && chgF == "true" {
		err := errors.New("not distinguish delete or change event")
		log.Error(err, "not distinguish delete or change event")
		return ctrl.Result{}, err
	}

	//delete事件
	if delFExists == true && delF == "true" {
		//校验
		result, targetPva, err := r.checkDelete(ctx, log, cp)
		if err != nil {
			return result, err
		}

		//1停止任务
		jobOld, ok := r.Jobs.Load(cp.GetNamespace() + "/" + cp.GetName())
		if ok == true {
			jobOld.(*cron.Cron).Stop()
			r.Jobs.Delete(cp.GetNamespace() + "/" + cp.GetName())
		}

		//2操作底层资源
		//（1）删除cronpva
		err = r.K8sDynamicClient.Resource(schema.GroupVersionResource{Group: "ipaas.icks.inspur.com", Version: "v1", Resource: "cronpersistentvolumeautoscalers"}).Namespace(cp.GetNamespace()).Delete(context.TODO(), cp.GetName(), metav1.DeleteOptions{})
		if err != nil {
			//回滚
			r.cpJobCreateOrUpdate(ctx, log, cp)
			err := errors.New("cp " + cp.Name + " delete error")
			log.Error(err, "cp "+cp.Name+" delete error")
			return ctrl.Result{}, err
		} else {
			//（2）目标pva增加delete-flag注解
			targetPvaAnnotations := targetPva.GetAnnotations()
			if targetPvaAnnotations == nil {
				targetPvaAnnotations = map[string]string{}
			}
			targetPvaAnnotations["delete-flag"] = "true"
			targetPva.SetAnnotations(targetPvaAnnotations)
			//操作失败不需要回滚，允许部分pva残留
			r.K8sDynamicClient.Resource(schema.GroupVersionResource{Group: "ipaas.icks.inspur.com", Version: "v1", Resource: "persistentvolumeautoscalers"}).Namespace(cp.GetNamespace()).Update(context.TODO(), targetPva, metav1.UpdateOptions{})
		}

		return ctrl.Result{}, nil
	}

	//change事件(定时-->阈值/智能)
	if chgFExists == true && chgF == "true" {
		//校验
		result, err := r.checkChange(ctx, log, cp)
		if err != nil {
			return result, err
		}

		//1停止任务
		jobOld, ok := r.Jobs.Load(cp.GetNamespace() + "/" + cp.GetName())
		if ok == true {
			jobOld.(*cron.Cron).Stop()
			r.Jobs.Delete(cp.GetNamespace() + "/" + cp.GetName())
		}

		//2操作底层cronpva资源
		r.K8sDynamicClient.Resource(schema.GroupVersionResource{Group: "ipaas.icks.inspur.com", Version: "v1", Resource: "cronpersistentvolumeautoscalers"}).Namespace(cp.GetNamespace()).Delete(context.TODO(), cp.GetName(), metav1.DeleteOptions{})

		return ctrl.Result{}, nil
	}

	//update事件
	result, err := r.checkCreateOrUpdate(ctx, log, cp)
	if err != nil {
		return result, err
	}

	r.cpJobCreateOrUpdate(ctx, log, cp)

	return ctrl.Result{}, nil
}

func (r *CronPersistentVolumeAutoscalerReconciler) checkCreateOrUpdate(ctx context.Context, log logr.Logger, cp *pvcv1.CronPersistentVolumeAutoscaler) (ctrl.Result, error) {
	//获取目标pva信息
	targetPvaName := cp.Spec.TargetRef.PvaName
	if utils.IsEmpty(targetPvaName) {
		err := errors.New("cp " + cp.Name + " target pva " + cp.Spec.TargetRef.PvaName + " name error")
		log.Error(err, "cp "+cp.Name+" target pva "+cp.Spec.TargetRef.PvaName+" name error")
		return ctrl.Result{}, err
	}

	targetPva, err := r.K8sDynamicClient.Resource(schema.GroupVersionResource{Group: "ipaas.icks.inspur.com", Version: "v1", Resource: "persistentvolumeautoscalers"}).Namespace(cp.GetNamespace()).Get(context.TODO(), targetPvaName, metav1.GetOptions{})
	if err != nil {
		err := errors.New("cp " + cp.Name + " target pva " + cp.Spec.TargetRef.PvaName + " underly error")
		log.Error(err, "cp "+cp.Name+" target pva "+cp.Spec.TargetRef.PvaName+" underly error")
		return ctrl.Result{}, err
	}
	if targetPva == nil {
		err := errors.New("cp " + cp.Name + " target pva " + cp.Spec.TargetRef.PvaName + " underly is not found")
		log.Error(err, "cp "+cp.Name+" target pva "+cp.Spec.TargetRef.PvaName+" underly is not found")
		return ctrl.Result{}, err
	}
	pvaUnstructuredContent := targetPva.UnstructuredContent()
	spec := pvaUnstructuredContent["spec"]

	//校验pva状态
	status := pvaUnstructuredContent["status"]
	phase := status.(map[string]interface{})["phase"].(string)
	if phase != "True" {
		err := errors.New("cp " + cp.Name + " target pva " + cp.Spec.TargetRef.PvaName + " status not true")
		log.Error(err, "cp "+cp.Name+" target pva "+cp.Spec.TargetRef.PvaName+" status not true")
		return ctrl.Result{}, err
	}

	//解析condition
	conditions := spec.(map[string]interface{})["conditions"]
	condition0 := conditions.([]interface{})[0].(map[string]interface{})
	condition0Type := condition0["type"].(string)
	if condition0Type != "volume-cron" {
		err := errors.New("cp " + cp.Name + " target pva condition type config error")
		log.Error(err, "cp "+cp.Name+" target pva condition type config error")
		return ctrl.Result{}, err
	}

	//解析action
	actions := spec.(map[string]interface{})["actions"]
	action0 := actions.([]interface{})[0].(map[string]interface{})
	action0Name := action0["name"].(string)
	if utils.IsEmpty(action0Name) {
		err := errors.New("cp " + cp.Name + " target pva action0Name is empty")
		log.Error(err, "cp "+cp.Name+" target pva action0Name is empty")
		return ctrl.Result{}, err
	}
	action0Params := action0["params"].(map[string]interface{})
	action0ParamScale := action0Params["scale"].(string)
	if utils.IsEmpty(action0ParamScale) {
		err := errors.New("cp " + cp.Name + " target pva action0ParamScale is empty")
		log.Error(err, "cp "+cp.Name+" target pva action0ParamScale is empty")
		return ctrl.Result{}, err
	}
	action0ParamLimits := action0Params["limits"].(string)
	if utils.IsEmpty(action0ParamLimits) {
		err := errors.New("cp " + cp.Name + " target pva action0ParamLimits is empty")
		log.Error(err, "cp "+cp.Name+" target pva action0ParamLimits is empty")
		return ctrl.Result{}, err
	}

	targetRefs := spec.(map[string]interface{})["targetRefs"].(map[string]interface{})
	pvcNames := targetRefs["pvcNames"].([]interface{})
	if len(pvcNames) < 1 {
		err := errors.New("cp " + cp.Name + " target pva pvcName is empty")
		log.Error(err, "cp "+cp.Name+" target pva pvcName is empty")
		return ctrl.Result{}, err
	}
	pvcName := pvcNames[0].(string)
	if utils.IsEmpty(action0ParamLimits) {
		err := errors.New("cp " + cp.Name + " target pva pvcName is empty")
		log.Error(err, "cp "+cp.Name+" target pva pvcName is empty")
		return ctrl.Result{}, err
	}

	//目标pvc
	pvc, err := r.K8sClient.CoreV1().PersistentVolumeClaims(targetPva.GetNamespace()).Get(context.TODO(), pvcName, metav1.GetOptions{})
	if err != nil {
		err := errors.New("cp " + cp.Name + " target pvc " + pvcName + " underly error")
		log.Error(err, "cp "+cp.Name+" target pvc "+pvcName+" underly error")
		return ctrl.Result{}, err
	}
	if pvc == nil {
		err := errors.New("cp " + cp.Name + " target pvc " + pvcName + " underly is not found")
		log.Error(err, "cp "+cp.Name+" target pvc "+pvcName+" underly is not found")
		return ctrl.Result{}, err
	}
	if pvc.Status.Phase != "Bound" {
		err := errors.New("cp " + cp.Name + " target pvc " + pvcName + " is not bound")
		log.Error(err, "cp "+cp.Name+" target pvc "+pvcName+" is not bound")
		return ctrl.Result{}, err
	}
	if utils.IsEmpty(pvc.Spec.VolumeName) {
		err := errors.New("cp " + cp.Name + " target pvc " + pvcName + " is not found pv")
		log.Error(err, "cp "+cp.Name+" target pvc "+pvcName+" is not found pv")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *CronPersistentVolumeAutoscalerReconciler) checkDelete(ctx context.Context, log logr.Logger, cp *pvcv1.CronPersistentVolumeAutoscaler) (ctrl.Result, *unstructured.Unstructured, error) {
	//获取目标pva信息
	targetPvaName := cp.Spec.TargetRef.PvaName
	if utils.IsEmpty(targetPvaName) {
		err := errors.New("cp " + cp.Name + " target pva " + cp.Spec.TargetRef.PvaName + " name error")
		log.Error(err, "cp "+cp.Name+" target pva "+cp.Spec.TargetRef.PvaName+" name error")
		return ctrl.Result{}, nil, err
	}

	targetPva, err := r.K8sDynamicClient.Resource(schema.GroupVersionResource{Group: "ipaas.icks.inspur.com", Version: "v1", Resource: "persistentvolumeautoscalers"}).Namespace(cp.GetNamespace()).Get(context.TODO(), targetPvaName, metav1.GetOptions{})
	if err != nil {
		err := errors.New("cp " + cp.Name + " target pva " + cp.Spec.TargetRef.PvaName + " underly error")
		log.Error(err, "cp "+cp.Name+" target pva "+cp.Spec.TargetRef.PvaName+" underly error")
		return ctrl.Result{}, nil, err
	}
	if targetPva == nil {
		err := errors.New("cp " + cp.Name + " target pva " + cp.Spec.TargetRef.PvaName + " underly is not found")
		log.Error(err, "cp "+cp.Name+" target pva "+cp.Spec.TargetRef.PvaName+" underly is not found")
		return ctrl.Result{}, nil, err
	}
	pvaUnstructuredContent := targetPva.UnstructuredContent()
	spec := pvaUnstructuredContent["spec"]

	//校验pva状态
	status := pvaUnstructuredContent["status"]
	phase := status.(map[string]interface{})["phase"].(string)
	if phase != "True" {
		err := errors.New("cp " + cp.Name + " target pva " + cp.Spec.TargetRef.PvaName + " status not true")
		log.Error(err, "cp "+cp.Name+" target pva "+cp.Spec.TargetRef.PvaName+" status not true")
		return ctrl.Result{}, nil, err
	}

	//解析condition
	conditions := spec.(map[string]interface{})["conditions"]
	condition0 := conditions.([]interface{})[0].(map[string]interface{})
	condition0Type := condition0["type"].(string)
	if condition0Type != "volume-cron" {
		err := errors.New("cp " + cp.Name + " target pva condition type config error")
		log.Error(err, "cp "+cp.Name+" target pva condition type config error")
		return ctrl.Result{}, nil, err
	}

	//解析action
	actions := spec.(map[string]interface{})["actions"]
	action0 := actions.([]interface{})[0].(map[string]interface{})
	action0Name := action0["name"].(string)
	if utils.IsEmpty(action0Name) {
		err := errors.New("cp " + cp.Name + " target pva action0Name is empty")
		log.Error(err, "cp "+cp.Name+" target pva action0Name is empty")
		return ctrl.Result{}, nil, err
	}
	action0Params := action0["params"].(map[string]interface{})
	action0ParamScale := action0Params["scale"].(string)
	if utils.IsEmpty(action0ParamScale) {
		err := errors.New("cp " + cp.Name + " target pva action0ParamScale is empty")
		log.Error(err, "cp "+cp.Name+" target pva action0ParamScale is empty")
		return ctrl.Result{}, nil, err
	}
	action0ParamLimits := action0Params["limits"].(string)
	if utils.IsEmpty(action0ParamLimits) {
		err := errors.New("cp " + cp.Name + " target pva action0ParamLimits is empty")
		log.Error(err, "cp "+cp.Name+" target pva action0ParamLimits is empty")
		return ctrl.Result{}, nil, err
	}

	targetRefs := spec.(map[string]interface{})["targetRefs"].(map[string]interface{})
	pvcNames := targetRefs["pvcNames"].([]interface{})
	if len(pvcNames) < 1 {
		err := errors.New("cp " + cp.Name + " target pva pvcName is empty")
		log.Error(err, "cp "+cp.Name+" target pva pvcName is empty")
		return ctrl.Result{}, nil, err
	}
	pvcName := pvcNames[0].(string)
	if utils.IsEmpty(action0ParamLimits) {
		err := errors.New("cp " + cp.Name + " target pva pvcName is empty")
		log.Error(err, "cp "+cp.Name+" target pva pvcName is empty")
		return ctrl.Result{}, nil, err
	}

	//目标pvc
	pvc, err := r.K8sClient.CoreV1().PersistentVolumeClaims(targetPva.GetNamespace()).Get(context.TODO(), pvcName, metav1.GetOptions{})
	if err != nil {
		err := errors.New("cp " + cp.Name + " target pvc " + pvcName + " underly error")
		log.Error(err, "cp "+cp.Name+" target pvc "+pvcName+" underly error")
		return ctrl.Result{}, nil, err
	}
	if pvc == nil {
		err := errors.New("cp " + cp.Name + " target pvc " + pvcName + " underly is not found")
		log.Error(err, "cp "+cp.Name+" target pvc "+pvcName+" underly is not found")
		return ctrl.Result{}, nil, err
	}
	if pvc.Status.Phase != "Bound" {
		err := errors.New("cp " + cp.Name + " target pvc " + pvcName + " is not bound")
		log.Error(err, "cp "+cp.Name+" target pvc "+pvcName+" is not bound")
		return ctrl.Result{}, nil, err
	}
	if utils.IsEmpty(pvc.Spec.VolumeName) {
		err := errors.New("cp " + cp.Name + " target pvc " + pvcName + " is not found pv")
		log.Error(err, "cp "+cp.Name+" target pvc "+pvcName+" is not found pv")
		return ctrl.Result{}, nil, err
	}

	return ctrl.Result{}, targetPva, nil
}

func (r *CronPersistentVolumeAutoscalerReconciler) checkChange(ctx context.Context, log logr.Logger, cp *pvcv1.CronPersistentVolumeAutoscaler) (ctrl.Result, error) {
	//获取目标pva信息
	return ctrl.Result{}, nil
}

func (r *CronPersistentVolumeAutoscalerReconciler) cpJobCreateOrUpdate(ctx context.Context, log logr.Logger, cp *pvcv1.CronPersistentVolumeAutoscaler) (ctrl.Result, error) {
	schedule := cp.Spec.CronTabs[0].Schedule
	if utils.IsEmpty(schedule) {
		err := errors.New("cp " + cp.Name + " target pva " + cp.Spec.TargetRef.PvaName + " schedule is empty")
		log.Error(err, "cp "+cp.Name+" target pva "+cp.Spec.TargetRef.PvaName+" schedule is empty")
		return ctrl.Result{}, err
	}
	//（1）停止原任务
	jobOld, ok := r.Jobs.Load(cp.GetNamespace() + "/" + cp.GetName())
	if ok == true {
		jobOld.(*cron.Cron).Stop()
		r.Jobs.Delete(cp.GetNamespace() + "/" + cp.GetName())
	}
	//（2）根据当前配置创建新任务
	c := cron.New()
	cj := CronJob{
		PvaName:          cp.Spec.TargetRef.PvaName,
		Namespace:        cp.GetNamespace(),
		K8sClient:        r.K8sClient,
		K8sDynamicClient: r.K8sDynamicClient,
		Recorder:         r.Recorder,
	}
	err := c.AddJob(schedule, cj)
	if err != nil {
		err := errors.New("cp " + cp.Name + " add job error")
		log.Error(err, "cp "+cp.Name+" add job error")
		return ctrl.Result{}, err
	}

	c.Start()
	r.Jobs.Store(cp.GetNamespace()+"/"+cp.GetName(), c)

	return ctrl.Result{}, nil
}

/*
*
事件记录、去重
*/
func (r *CronPersistentVolumeAutoscalerReconciler) pvaRecordEvent(cp *pvcv1.CronPersistentVolumeAutoscaler, eventType, eventReason, eventMessage string) {
	defer func() {
		if err := recover(); err != nil {
			utils.Logger.Error(fmt.Sprintf("pva record event error: %v", err))
		}
	}()

	listOptions := metav1.ListOptions{}
	listOptions.Kind = "Event"
	fieldSelector := make(map[string]string)
	fieldSelector["involvedObject.name"] = cp.GetName()
	fieldSelector["involvedObject.kind"] = cp.Kind
	fieldSelector["involvedObject.namespace"] = cp.GetNamespace()
	fieldSelector["involvedObject.uid"] = utils.ToString(cp.GetUID())
	listOptions.FieldSelector = labels.FormatLabels(fieldSelector)

	eventList, err := r.K8sClient.CoreV1().Events(cp.GetNamespace()).List(context.Background(), listOptions)
	if err == nil && eventList != nil && len(eventList.Items) > 0 {
		for _, event := range eventList.Items {
			if event.Type == eventType && event.Reason == eventReason && event.Message == eventMessage {
				r.K8sClient.CoreV1().Events(event.GetNamespace()).Delete(context.Background(), event.GetName(), metav1.DeleteOptions{})
			}
		}

	}

	r.Recorder.Eventf(cp, eventType, eventReason, eventMessage)
}

/*
*
事件记录、包含重复事件
*/
func (r *CronPersistentVolumeAutoscalerReconciler) pvaRecordEventOriginal(cp *pvcv1.CronPersistentVolumeAutoscaler, eventType, eventReason, eventMessage string) {
	defer func() {
		if err := recover(); err != nil {
			utils.Logger.Error(fmt.Sprintf("pva record event error: %v", err))
		}
	}()

	r.Recorder.Eventf(cp, eventType, eventReason, eventMessage)
}
