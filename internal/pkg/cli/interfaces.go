// Copyright Amazon.com Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"encoding"
	"io"

	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/copilot-cli/internal/pkg/aws/cloudformation"
	"github.com/aws/copilot-cli/internal/pkg/aws/cloudwatchlogs"
	"github.com/aws/copilot-cli/internal/pkg/aws/codepipeline"
	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/aws/copilot-cli/internal/pkg/deploy"
	"github.com/aws/copilot-cli/internal/pkg/deploy/cloudformation/stack"
	"github.com/aws/copilot-cli/internal/pkg/describe"
	"github.com/aws/copilot-cli/internal/pkg/docker"
	"github.com/aws/copilot-cli/internal/pkg/docker/dockerfile"
	"github.com/aws/copilot-cli/internal/pkg/repository"
	"github.com/aws/copilot-cli/internal/pkg/task"
	"github.com/aws/copilot-cli/internal/pkg/term/command"
	"github.com/aws/copilot-cli/internal/pkg/term/selector"
	"github.com/aws/copilot-cli/internal/pkg/workspace"
)

// actionCommand is the interface that every command that creates a resource implements.
type actionCommand interface {
	// Validate returns an error if a flag's value is invalid.
	Validate() error

	// Ask prompts for flag values that are required but not passed in.
	Ask() error

	// Execute runs the command after collecting all required options.
	Execute() error

	// RecommendedActions returns a list of follow-up suggestions users can run once the command executes successfully.
	RecommendedActions() []string
}

// SSM store interface.

type serviceStore interface {
	serviceCreator
	serviceGetter
	serviceLister
	serviceDeleter
}

type serviceCreator interface {
	CreateService(svc *config.Service) error
}

type serviceGetter interface {
	GetService(appName, svcName string) (*config.Service, error)
}

type serviceLister interface {
	ListServices(appName string) ([]*config.Service, error)
}

type serviceDeleter interface {
	DeleteService(appName, svcName string) error
}

type applicationStore interface {
	applicationCreator
	applicationGetter
	applicationLister
	applicationDeleter
}

type applicationCreator interface {
	CreateApplication(app *config.Application) error
}

type applicationGetter interface {
	GetApplication(appName string) (*config.Application, error)
}

type applicationLister interface {
	ListApplications() ([]*config.Application, error)
}

type applicationDeleter interface {
	DeleteApplication(name string) error
}

type environmentStore interface {
	environmentCreator
	environmentGetter
	environmentLister
	environmentDeleter
}

type environmentCreator interface {
	CreateEnvironment(env *config.Environment) error
}

type environmentGetter interface {
	GetEnvironment(appName string, environmentName string) (*config.Environment, error)
}

type environmentLister interface {
	ListEnvironments(appName string) ([]*config.Environment, error)
}

type environmentDeleter interface {
	DeleteEnvironment(appName, environmentName string) error
}

type store interface {
	applicationStore
	environmentStore
	serviceStore
}

type deployedEnvironmentLister interface {
	ListEnvironmentsDeployedTo(appName, svcName string) ([]string, error)
	ListDeployedServices(appName, envName string) ([]string, error)
	IsServiceDeployed(appName, envName string, svcName string) (bool, error)
}

// Secretsmanager interface.

type secretsManager interface {
	secretCreator
	secretDeleter
}

type secretCreator interface {
	CreateSecret(secretName, secretString string) (string, error)
}

type secretDeleter interface {
	DeleteSecret(secretName string) error
}

type imageBuilderPusher interface {
	BuildAndPush(docker repository.ContainerLoginBuildPusher, args *docker.BuildArguments) error
}

type repositoryURIGetter interface {
	URI() string
}

type repositoryService interface {
	repositoryURIGetter
	imageBuilderPusher
}

type cwlogService interface {
	TaskLogEvents(logGroupName string, streamLastEventTime map[string]int64, opts ...cloudwatchlogs.GetLogEventsOpts) (*cloudwatchlogs.LogEventsOutput, error)
	LogGroupExists(logGroupName string) (bool, error)
}

type templater interface {
	Template() (string, error)
}

type stackSerializer interface {
	templater
	SerializedParameters() (string, error)
}

type runner interface {
	Run(name string, args []string, options ...command.Option) error
}

type eventsWriter interface {
	WriteEventsUntilStopped() error
}

type defaultSessionProvider interface {
	Default() (*session.Session, error)
}

type regionalSessionProvider interface {
	DefaultWithRegion(region string) (*session.Session, error)
}

type sessionFromRoleProvider interface {
	FromRole(roleARN string, region string) (*session.Session, error)
}

type sessionFromStaticProvider interface {
	FromStaticCreds(accessKeyID, secretAccessKey, sessionToken string) (*session.Session, error)
}

type sessionFromProfileProvider interface {
	FromProfile(name string) (*session.Session, error)
}

type profileNames interface {
	Names() []string
}

type sessionProvider interface {
	defaultSessionProvider
	regionalSessionProvider
	sessionFromRoleProvider
	sessionFromProfileProvider
	sessionFromStaticProvider
}

type describer interface {
	Describe() (describe.HumanJSONStringer, error)
}

type wsFileDeleter interface {
	DeleteWorkspaceFile() error
}

type svcManifestReader interface {
	ReadServiceManifest(svcName string) ([]byte, error)
}

type svcManifestWriter interface {
	WriteServiceManifest(marshaler encoding.BinaryMarshaler, svcName string) (string, error)
}

type wsPipelineManifestReader interface {
	ReadPipelineManifest() ([]byte, error)
}

type wsPipelineWriter interface {
	WritePipelineBuildspec(marshaler encoding.BinaryMarshaler) (string, error)
	WritePipelineManifest(marshaler encoding.BinaryMarshaler) (string, error)
}

type wsServiceLister interface {
	ServiceNames() ([]string, error)
}

type wsSvcReader interface {
	wsServiceLister
	svcManifestReader
}

type wsSvcDirReader interface {
	wsSvcReader
	CopilotDirPath() (string, error)
}

type wsPipelineReader interface {
	wsServiceLister
	wsPipelineManifestReader
}

type wsAppManager interface {
	Create(appName string) error
	Summary() (*workspace.Summary, error)
}

type wsAddonManager interface {
	WriteAddon(f encoding.BinaryMarshaler, svc, name string) (string, error)
	wsSvcReader
}

type artifactUploader interface {
	PutArtifact(bucket, fileName string, data io.Reader) (string, error)
}

type bucketEmptier interface {
	EmptyBucket(bucket string) error
}

// Interfaces for deploying resources through CloudFormation. Facilitates mocking.
type environmentDeployer interface {
	DeployEnvironment(env *deploy.CreateEnvironmentInput) error
	StreamEnvironmentCreation(env *deploy.CreateEnvironmentInput) (<-chan []deploy.ResourceEvent, <-chan deploy.CreateEnvironmentResponse)
	DeleteEnvironment(appName, envName string) error
	GetEnvironment(appName, envName string) (*config.Environment, error)
}

type svcDeleter interface {
	DeleteService(in deploy.DeleteServiceInput) error
}

type svcRemoverFromApp interface {
	RemoveServiceFromApp(app *config.Application, svcName string) error
}

type imageRemover interface {
	ClearRepository(repoName string) error // implemented by ECR Service
}

type pipelineDeployer interface {
	CreatePipeline(env *deploy.CreatePipelineInput) error
	UpdatePipeline(env *deploy.CreatePipelineInput) error
	PipelineExists(env *deploy.CreatePipelineInput) (bool, error)
	DeletePipeline(pipelineName string) error
	AddPipelineResourcesToApp(app *config.Application, region string) error
	appResourcesGetter
	// TODO: Add StreamPipelineCreation method
}

type appDeployer interface {
	DeployApp(in *deploy.CreateAppInput) error
	AddServiceToApp(app *config.Application, svcName string) error
	AddEnvToApp(app *config.Application, env *config.Environment) error
	DelegateDNSPermissions(app *config.Application, accountID string) error
	DeleteApp(name string) error
}

type appResourcesGetter interface {
	GetAppResourcesByRegion(app *config.Application, region string) (*stack.AppRegionalResources, error)
	GetRegionalAppResources(app *config.Application) ([]*stack.AppRegionalResources, error)
}

type taskDeployer interface {
	DeployTask(input *deploy.CreateTaskResourcesInput, opts ...cloudformation.StackOption) error
}

type taskRunner interface {
	Run() ([]*task.Task, error)
}

type defaultClusterGetter interface {
	HasDefaultCluster() (bool, error)
}

type deployer interface {
	environmentDeployer
	appDeployer
	pipelineDeployer
}

type domainValidator interface {
	DomainExists(domainName string) (bool, error)
}

type dockerfileParser interface {
	GetExposedPorts() ([]uint16, error)
	GetHealthCheck() (*dockerfile.HealthCheck, error)
}

type statusDescriber interface {
	Describe() (*describe.ServiceStatusDesc, error)
}

type envDescriber interface {
	Describe() (*describe.EnvDescription, error)
}

type pipelineGetter interface {
	GetPipeline(pipelineName string) (*codepipeline.Pipeline, error)
	ListPipelineNamesByTags(tags map[string]string) ([]string, error)
}

type executor interface {
	Execute() error
}

type deletePipelineRunner interface {
	Run() error
}

type askExecutor interface {
	Ask() error
	executor
}

type appSelector interface {
	Application(prompt, help string, additionalOpts ...string) (string, error)
}

type appEnvSelector interface {
	appSelector
	Environment(prompt, help, app string, additionalOpts ...string) (string, error)
}

type configSelector interface {
	appEnvSelector
	Service(prompt, help, app string) (string, error)
}

type deploySelector interface {
	appSelector
	DeployedService(prompt, help string, app string, opts ...selector.GetDeployedServiceOpts) (*selector.DeployedService, error)
}

type wsSelector interface {
	appEnvSelector
	Service(prompt, help string) (string, error)
}

type ec2Selector interface {
	VPC(prompt, help string) (string, error)
	PublicSubnets(prompt, help, vpcID string) ([]string, error)
	PrivateSubnets(prompt, help, vpcID string) ([]string, error)
}

type credsSelector interface {
	Creds(prompt, help string) (*session.Session, error)
}
