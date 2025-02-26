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

package main

import (
	"flag"
	"fmt"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"os"
	"pvc-operator/api/v1"
	"pvc-operator/constants"
	"pvc-operator/utils"
	"strconv"
	"strings"
	"sync"

	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"pvc-operator/controllers"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	_ = clientgoscheme.AddToScheme(scheme)
	_ = v1.AddToScheme(scheme)
	// +kubebuilder:scaffold:scheme
}

func main() {
	var metricsAddr string
	var enableLeaderElection bool
	utils.Logger.Info(fmt.Sprintf("CP_OPERATOR_RUN_MODE: %s", constants.CP_OPERATOR_RUN_MODE))
	flag.StringVar(&metricsAddr, "metrics-addr", ":8082", "The address the metric endpoint binds to.")

	//处理环境变量
	envs := os.Environ()
	cpEnvs := map[string]string{}
	for _, env := range envs {
		envArr := strings.Split(env, "=")
		if len(envArr) < 2 {
			continue
		}
		//是否选主
		if "enable-leader-election" == strings.Split(env, "=")[0] {
			cpEnvs["enable-leader-election"] = strings.Split(env, "=")[1]
		}
	}
	utils.Logger.Info("system envs:", cpEnvs)
	enableLeaderElection = true
	if v, ok := cpEnvs["enable-leader-election"]; ok {
		enableLeaderElectionStr := strings.ReplaceAll(v, "\"", "")
		enableLeaderElection, _ = strconv.ParseBool(enableLeaderElectionStr)
	}
	flag.BoolVar(&enableLeaderElection, "enable-leader-election", enableLeaderElection,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")

	flag.Parse()

	ctrl.SetLogger(zap.New(func(o *zap.Options) {
		o.Development = true
	}))

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                  scheme,
		MetricsBindAddress:      metricsAddr,
		LeaderElection:          enableLeaderElection,
		LeaderElectionID:        "cp.ipaas.icks.inspur.com",
		LeaderElectionNamespace: "kube-system",
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	if err = (&controllers.CronPersistentVolumeAutoscalerReconciler{
		Client:   mgr.GetClient(),
		Log:      ctrl.Log.WithName("controllers").WithName("CronPersistentVolumeAutoscaler"),
		Scheme:   mgr.GetScheme(),
		Recorder: mgr.GetEventRecorderFor("CronPersistentVolumeAutoscaler"),
		CpEnvs:   cpEnvs,
		Jobs:     sync.Map{},
		K8sClient: func() kubernetes.Interface {
			config, err := rest.InClusterConfig()
			if err != nil {
				setupLog.Error(err, "rest InClusterConfig error")
				os.Exit(1)
			}
			k8sClient, err := kubernetes.NewForConfig(config)
			if err != nil {
				setupLog.Error(err, "init k8sClient error")
				os.Exit(1)
			}
			return k8sClient
		}(),
		K8sDynamicClient: func() dynamic.Interface {
			config, err := rest.InClusterConfig()
			if err != nil {
				setupLog.Error(err, "rest InClusterConfig error")
				os.Exit(1)
			}
			K8sDynamicClient, err := dynamic.NewForConfig(config)
			if err != nil {
				setupLog.Error(err, "init K8sDynamicClient error")
				os.Exit(1)
			}
			return K8sDynamicClient
		}(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "CronPersistentVolumeAutoscaler")
		os.Exit(1)
	}
	// +kubebuilder:scaffold:builder

	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}
