package runner

import (
	"fmt"
	"go.opentelemetry.io/auto/pkg/errors"
	"go.opentelemetry.io/auto/pkg/instrumentors"
	"go.opentelemetry.io/auto/pkg/log"
	"go.opentelemetry.io/auto/pkg/opentelemetry"
	"go.opentelemetry.io/auto/pkg/process"
)

type Runner struct {
	job             process.JobConfig
	target          *process.TargetArgs
	processAnalyzer *process.Analyzer
	instManager     *instrumentors.Manager
}

func NewRunner(job process.JobConfig) Runner {
	log.Logger.V(0).Info(fmt.Sprintf("starting Go OpenTelemetry Agent for exec path %s", job.BinaryPath))

	// TODO Change this part so it's get data from file instead of from ENV
	target := process.ParseTargetArgs(job.BinaryPath)
	if err := target.Validate(); err != nil {
		log.Logger.Error(err, "invalid target args")
		panic(err)
	}

	processAnalyzer := process.NewAnalyzer()
	otelController, err := opentelemetry.NewController(job.ServiceName)
	if err != nil {
		log.Logger.Error(err, "unable to create OpenTelemetry controller")
		panic(err)
	}

	instManager, err := instrumentors.NewManager(otelController)
	if err != nil {
		log.Logger.Error(err, "error creating instrumetors manager")
		panic(err)
	}

	return Runner{
		job:             job,
		target:          target,
		processAnalyzer: processAnalyzer,
		instManager:     instManager,
	}
}

func (r *Runner) Run() error {
	pid, err := r.processAnalyzer.DiscoverProcessID(r.target)
	if err != nil {
		if err != errors.ErrInterrupted {
			log.Logger.Error(err, "error while discovering process id")
		}
		return err
	}

	targetDetails, err := r.processAnalyzer.Analyze(pid, r.instManager.GetRelevantFuncs())
	if err != nil {
		log.Logger.Error(err, "error while analyzing target process")
		return err
	}
	log.Logger.V(0).Info("target process analysis completed", "pid", targetDetails.PID,
		"go_version", targetDetails.GoVersion, "dependencies", targetDetails.Libraries,
		"total_functions_found", len(targetDetails.Functions))

	r.instManager.FilterUnusedInstrumentors(targetDetails)

	log.Logger.V(0).Info("invoking instrumentors")
	err = r.instManager.Run(targetDetails)
	if err != nil && err != errors.ErrInterrupted {
		log.Logger.Error(err, "error while running instrumentors")
		return err
	}

	return nil
}

func (r *Runner) Close() {
	log.Logger.V(0).Info("Got SIGTERM, cleaning up..")
	r.processAnalyzer.Close()
	r.instManager.Close()
}
