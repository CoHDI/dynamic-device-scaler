/*
Copyright 2025 The CoHDI Authors.

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
	"os"
	"strconv"
	"time"

	"k8s.io/client-go/kubernetes"
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	cdioperator "github.com/IBM/composable-resource-operator/api/v1alpha1"
	"go.uber.org/zap/zapcore"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"github.com/CoHDI/dynamic-device-scaler/internal/controller"
	// +kubebuilder:scaffold:imports
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	_ = clientgoscheme.AddToScheme(scheme)
	_ = cdioperator.AddToScheme(scheme)

	// +kubebuilder:scaffold:scheme
}

// nolint:gocyclo
func main() {
	const timeoutDuration = 30 * time.Second

	var enableLeaderElection bool
	var probeAddr, logLevel string

	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	flag.StringVar(&logLevel, "log-level", "info", "Set the logging level")

	flag.Parse()

	var level zapcore.Level
	switch logLevel {
	case "debug":
		level = zapcore.DebugLevel
	case "info":
		level = zapcore.InfoLevel
	case "error":
		level = zapcore.ErrorLevel
	default:
		level = zapcore.InfoLevel
	}

	opts := zap.Options{
		Level: level,
	}

	opts.BindFlags(flag.CommandLine)

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	cfg := ctrl.GetConfigOrDie()

	cfg.Timeout = timeoutDuration

	mgr, err := ctrl.NewManager(cfg, ctrl.Options{
		Scheme:                 scheme,
		HealthProbeBindAddress: probeAddr,
		LeaderElection:         enableLeaderElection,
		LeaderElectionID:       "2313b367.co.hdi",
		// LeaderElectionReleaseOnCancel defines if the leader should step down voluntarily
		// when the Manager ends. This requires the binary to immediately end when the
		// Manager is stopped, otherwise, this setting is unsafe. Setting this significantly
		// speeds up voluntary leader transitions as the new leader don't have to wait
		// LeaseDuration time first.
		//
		// In the default scaffold provided, the program ends immediately after
		// the manager stops, so would be fine to enable this option. However,
		// if you are doing or is intended to do any operation such as perform cleanups
		// after the manager stops then its usage might be unsafe.
		// LeaderElectionReleaseOnCancel: true,
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	clientSet, err := kubernetes.NewForConfig(mgr.GetConfig())
	if err != nil {
		setupLog.Error(err, "unable to create clientSet", "controller", "ComposabilityRequest")
		os.Exit(1)
	}

	scanInterval, err := getEnvAsInt("SCAN_INTERVAL", 60)
	if err != nil {
		setupLog.Error(err, "invalid SCAN_INTERVAL")
		os.Exit(1)
	}

	deviceNoRemoval, err := getEnvAsInt("DEVICE_NO_REMOVAL_DURATION", 600)
	if err != nil {
		setupLog.Error(err, "invalid DEVICE_NO_REMOVAL_DURATION")
		os.Exit(1)
	}

	deviceNoAllocation, err := getEnvAsInt("DEVICE_NO_ALLOCATION_DURATION", 60)
	if err != nil {
		setupLog.Error(err, "invalid DEVICE_NO_ALLOCATION_DURATION")
		os.Exit(1)
	}

	setupLog.Info("Loaded environment variables",
		"SCAN_INTERVAL", scanInterval,
		"DEVICE_NO_REMOVAL_DURATION", deviceNoRemoval,
		"DEVICE_NO_ALLOCATION_DURATION", deviceNoAllocation)

	if err = (&controller.ResourceMonitorReconciler{
		Client:             mgr.GetClient(),
		ClientSet:          clientSet,
		Scheme:             mgr.GetScheme(),
		ScanInterval:       time.Duration(scanInterval) * time.Second,
		DeviceNoRemoval:    time.Duration(deviceNoRemoval) * time.Second,
		DeviceNoAllocation: time.Duration(deviceNoAllocation) * time.Second,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "ResourceMonitor")
		os.Exit(1)
	}
	// +kubebuilder:scaffold:builder

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up ready check")
		os.Exit(1)
	}

	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}

func getEnvAsInt(name string, defaultValue int) (int, error) {
	valueStr := os.Getenv(name)
	if valueStr == "" {
		return defaultValue, nil
	}

	value, err := strconv.Atoi(valueStr)
	if err != nil {
		return 0, fmt.Errorf("invalid integer value for %s: %v", name, err)
	}

	if value < 0 || value > 86400 {
		return 0, fmt.Errorf("%s must be between 0-86400, got %d", name, value)
	}

	return value, nil
}
