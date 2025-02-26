package controllers

import (
	"context"
	"fmt"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/record"
	"math"
	"pvc-operator/utils"
)

type CronJob struct {
	PvaName          string
	Namespace        string
	K8sClient        kubernetes.Interface
	K8sDynamicClient dynamic.Interface
	Recorder         record.EventRecorder
}

func (c CronJob) Run() {
	utils.Logger.Info(fmt.Sprintf("------------------begin excute cronpva %s job-------------------", c.PvaName))
	defer func() {
		if err := recover(); err != nil {
			utils.Logger.Error(fmt.Sprintf("excute cronpva job error: %v", err))
		}
	}()

	var eventType, eventReason, eventMessage string

	//获取pva信息
	targetPva, err := c.K8sDynamicClient.Resource(schema.GroupVersionResource{Group: "ipaas.icks.inspur.com", Version: "v1", Resource: "persistentvolumeautoscalers"}).Namespace(c.Namespace).Get(context.TODO(), c.PvaName, metav1.GetOptions{})
	if err != nil {
		utils.Logger.Error(fmt.Sprintf("get targetPva %s error:%v", c.PvaName, err))
		return
	}

	pvaUnstructuredContent := targetPva.UnstructuredContent()
	spec := pvaUnstructuredContent["spec"]
	//解析condition
	conditions := spec.(map[string]interface{})["conditions"]
	condition0 := conditions.([]interface{})[0].(map[string]interface{})
	condition0Type := condition0["type"].(string)
	utils.Logger.Info(fmt.Sprintf("pva %s condition-0-type:%s", c.PvaName, condition0Type))
	condition0Value := condition0["value"].(string)
	utils.Logger.Info(fmt.Sprintf("pva %s condition-0-value:%s", c.PvaName, condition0Value))
	if condition0Type == "volume-cron" {
		eventType = v1.EventTypeNormal
		eventReason = "SuccessfulParseCondition"
		eventMessage = "pva " + targetPva.GetName() + " condition parse completed"
		utils.Logger.Info(fmt.Sprintf(eventMessage))
		c.pvaRecordEventOriginal(targetPva, eventType, eventReason, eventMessage)
	} else {
		eventType = v1.EventTypeWarning
		eventReason = "FailedParseCondition"
		eventMessage = "pva " + targetPva.GetName() + " condition parse error"
		utils.Logger.Error(fmt.Sprintf(eventMessage))
		c.pvaRecordEventOriginal(targetPva, eventType, eventReason, eventMessage)
		return
	}
	//解析action
	actions := spec.(map[string]interface{})["actions"]
	action0 := actions.([]interface{})[0].(map[string]interface{})
	action0Name := action0["name"].(string)
	utils.Logger.Info(fmt.Sprintf("cron-pva %s action-0-name:%s", c.PvaName, action0Name))
	action0type := action0["type"].(string)
	utils.Logger.Info(fmt.Sprintf("cron-pva %s action-0-type:%s", c.PvaName, action0type))
	action0Params := action0["params"].(map[string]interface{})
	action0ParamScale := action0Params["scale"].(string)
	utils.Logger.Info(fmt.Sprintf("cron-pva %s action-0-param-scale:%s", c.PvaName, action0ParamScale))
	action0ParamLimits := action0Params["limits"].(string)
	utils.Logger.Info(fmt.Sprintf("cron-pva %s action-0-param-limits:%s", c.PvaName, action0ParamLimits))
	//解析pvc
	targetRefs := spec.(map[string]interface{})["targetRefs"].(map[string]interface{})
	pvcNames := targetRefs["pvcNames"].([]interface{})
	pvcName := pvcNames[0].(string)
	utils.Logger.Info(fmt.Sprintf("cron-pva %s action-0-param-pvcName:%s", c.PvaName, pvcName))

	//获取pvc信息
	pvc, err := c.K8sClient.CoreV1().PersistentVolumeClaims(targetPva.GetNamespace()).Get(context.TODO(), pvcName, metav1.GetOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		utils.Logger.Error(fmt.Sprintf("get targetPvc %s error:%v", pvcName, err))
		return
	}
	if apierrors.IsNotFound(err) || pvc == nil {
		eventType = v1.EventTypeWarning
		eventReason = "FailedCheckTargetPersistentVolumeClaimExists"
		eventMessage = "not found target pvc " + pvc.GetName()
		utils.Logger.Error(fmt.Sprintf(eventMessage))
		c.pvaRecordEventOriginal(targetPva, eventType, eventReason, eventMessage)
		return
	}
	if pvc.Status.Phase != "Bound" {
		eventType = v1.EventTypeWarning
		eventReason = "FailedCheckTargetPersistentVolumeClaimBound"
		eventMessage = "target pvc " + pvc.GetName() + " not bound pv"
		utils.Logger.Error(fmt.Sprintf(eventMessage))
		c.pvaRecordEventOriginal(targetPva, eventType, eventReason, eventMessage)
		return
	}

	utils.Logger.Info(fmt.Sprintf("cron-pva %s pvcStorage:%s", c.PvaName, pvc.Spec.Resources.Requests.Storage().String()))

	capacityResized := ""
	//固定容量
	if action0Name == "fixed-action" {
		capacityResized = utils.ToString(utils.ToInt(pvc.Spec.Resources.Requests.Storage().String()[0:len(pvc.Spec.Resources.Requests.Storage().String())-2])+utils.ToInt(action0ParamScale[0:len(action0ParamScale)-2])) + "Gi"
		//固定百分比
	} else if action0Name == "percentage-action" {
		capacityResized = utils.ToString(math.Ceil(utils.ToFloat(pvc.Spec.Resources.Requests.Storage().String()[0:len(pvc.Spec.Resources.Requests.Storage().String())-2])*(1+utils.ToFloat(action0ParamScale[0:len(action0ParamScale)-1])/100))) + "Gi"
	} else {
		eventType = v1.EventTypeWarning
		eventReason = "FailedParseAction"
		eventMessage = "pva " + targetPva.GetName() + " action parse error"
		utils.Logger.Error(fmt.Sprintf(eventMessage))
		c.pvaRecordEventOriginal(targetPva, eventType, eventReason, eventMessage)
		return
	}
	utils.Logger.Info(fmt.Sprintf("cron-pva %s pvcStorage:%s", c.PvaName, pvc.Spec.Resources.Requests.Storage().String()))

	//是否小于容量上限
	if utils.ToInt(capacityResized[0:len(capacityResized)-2]) <= utils.ToInt(action0ParamLimits[0:len(action0ParamLimits)-2]) {
		annotations := targetPva.GetAnnotations()
		if annotations == nil {
			annotations = map[string]string{}
		}
		//targetpvc-capacity/pvcname: 20Gi
		annotations["targetpvc-capacity/"+pvc.GetName()] = capacityResized
		targetPva.SetAnnotations(annotations)
		_, err = c.K8sDynamicClient.Resource(schema.GroupVersionResource{Group: "ipaas.icks.inspur.com", Version: "v1", Resource: "persistentvolumeautoscalers"}).Namespace(targetPva.GetNamespace()).Update(context.TODO(), targetPva, metav1.UpdateOptions{})
		if err != nil {
			utils.Logger.Error(fmt.Sprintf("set targetPva %s annotations error:%v", targetPva, err))
		} else {
			eventType = v1.EventTypeNormal
			eventReason = "SuccessfulTriggerExpand"
			eventMessage = "pva " + targetPva.GetName() + " trigger pvc " + pvc.GetName() + " expand successful and expected capacity is " + capacityResized
			utils.Logger.Info(fmt.Sprintf(eventMessage))
			c.pvaRecordEventOriginal(targetPva, eventType, eventReason, eventMessage)
		}
	} else {
		eventType = v1.EventTypeWarning
		eventReason = "FailedCheckTargetStorage"
		eventMessage = "target capacity is exceeded max value"
		utils.Logger.Error(fmt.Sprintf(eventMessage))
		c.pvaRecordEventOriginal(targetPva, eventType, eventReason, eventMessage)
	}

	utils.Logger.Info(fmt.Sprintf("------------------end excute cronpva %s job-------------------", c.PvaName))
}

/**
事件记录、包含重复事件
*/
func (c CronJob) pvaRecordEventOriginal(pva *unstructured.Unstructured, eventType, eventReason, eventMessage string) {
	defer func() {
		if err := recover(); err != nil {
			utils.Logger.Error(fmt.Sprintf("pva record event error: %v", err))
		}
	}()

	c.Recorder.Eventf(pva, eventType, eventReason, eventMessage)
}
