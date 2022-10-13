package main

import (
	"context"
	"flag"
	"os"
	"path/filepath"
	"time"

	"github.com/jetbrains-infra/deploy-problem-detector/pkg/client/k8s"
	"github.com/jetbrains-infra/deploy-problem-detector/pkg/formatter/teamcity"
	"github.com/jetbrains-infra/deploy-problem-detector/pkg/problemdetector"
	log "github.com/sirupsen/logrus"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
)

func main() {
	var kubeconfig *string

	if os.Getenv("TEAMCITY_VERSION") != "" {
		log.SetFormatter(&teamcity.TeamcityFormatter{})
	} else {
		log.SetFormatter(&log.TextFormatter{ForceColors: true, DisableTimestamp: true})
	}

	if home := homedir.HomeDir(); home != "" {
		kubeconfig = flag.String("kubeconfig", filepath.Join(home, ".kube", "config"), "(optional) absolute path to the kubeconfig file")
	} else {
		kubeconfig = flag.String("kubeconfig", "", "absolute path to the kubeconfig file")
	}
	var deploymentName = flag.String("name", "", "name of the deployment to track")
	var namespace = flag.String("namespace", "", "namespace of the deployment to track")
	flag.Parse()

	if deploymentName == nil || *deploymentName == "" {
		log.Fatalf("Please specify name of the deployment")
	}

	if namespace == nil || *namespace == "" {
		log.Fatalf("Please specify namespace of the deployment")
	}

	// use the current context in kubeconfig
	config, err := clientcmd.BuildConfigFromFlags("", *kubeconfig)
	if err != nil {
		log.Fatal(err)
	}

	client, err := k8s.New(*namespace, config)
	if err != nil {
		log.Fatal(err)
	}

	pd := problemdetector.New(client)

	ctx := context.Background()

	for {
		d, err := client.GetDeployment(ctx, *deploymentName)
		if err != nil {
			log.Fatal(err)
		}

		if pd.RolloutComplete(d) {
			break
		}

		if err := pd.StreamLogs(ctx, d); err != nil {
			log.Warn(err)
		}

		if err := pd.QuotaProblem(d); err != nil {
			log.Warn(err)
		}

		if err := pd.ContainersStartProblems(ctx, d); err != nil {
			log.Warn(err)
		}

		if err := pd.ReadinessProblem(ctx, d); err != nil {
			log.Warn(err)
		}

		if pd.DeployTimeout(d) {
			log.WithField(teamcity.MessageName, teamcity.MessageNameBuildProblem).Fatal("Deployment progress deadline exceeded")
		}

		time.Sleep(5 * time.Second)
	}

	log.WithField(teamcity.MessageName, teamcity.MessageNameBuildStatus).Printf("Deployment %q is successful", *deploymentName)
}
