package command

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/Sirupsen/logrus"
	"github.com/mitchellh/cli"
	v1auth "github.com/nerdalize/nerd/nerd/client/auth/v1"
	"github.com/nerdalize/nerd/nerd/jwt"
	"github.com/nerdalize/nerd/nerd/oauth"
	"github.com/pkg/errors"
)

//WorkerStartOpts describes command options
type WorkerStartOpts struct {
	Env []string `long:"env" short:"e" description:"environment variables"`
}

//WorkerStart command
type WorkerStart struct {
	*command
	opts *WorkerStartOpts
}

//WorkerStartFactory returns a factory method for the join command
func WorkerStartFactory() (cli.Command, error) {
	opts := &WorkerStartOpts{}
	comm, err := newCommand("nerd worker start <image>", "provision a new worker to provide compute", "", opts)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create command")
	}
	cmd := &WorkerStart{
		command: comm,
		opts:    opts,
	}
	cmd.runFunc = cmd.DoRun

	return cmd, nil
}

//DoRun is called by run and allows an error to be returned
func (cmd *WorkerStart) DoRun(args []string) (err error) {
	if len(args) < 1 {
		return fmt.Errorf("not enough arguments, see --help")
	}

	//fetching a worker JWT
	authbase, err := url.Parse(cmd.config.Auth.APIEndpoint)
	if err != nil {
		HandleError(errors.Wrapf(err, "auth endpoint '%v' is not a valid URL", cmd.config.Auth.APIEndpoint))
	}

	authOpsClient := v1auth.NewOpsClient(v1auth.OpsClientConfig{
		Base:   authbase,
		Logger: logrus.StandardLogger(),
	})

	authclient := v1auth.NewClient(v1auth.ClientConfig{
		Base:               authbase,
		Logger:             logrus.StandardLogger(),
		OAuthTokenProvider: oauth.NewConfigProvider(authOpsClient, cmd.config.Auth.ClientID, cmd.session),
	})

	ss, err := cmd.session.Read()
	if err != nil {
		HandleError(err)
	}

	workerJWT, err := authclient.GetWorkerJWT(ss.Project.Name, v1auth.NCEScope)
	if err != nil {
		HandleError(errors.Wrap(err, "failed to get worker JWT"))
	}

	bclient, err := NewClient(cmd.ui, cmd.config, cmd.session)
	if err != nil {
		HandleError(err)
	}

	wenv := map[string]string{}
	for _, l := range cmd.opts.Env {
		split := strings.SplitN(l, "=", 2)
		if len(split) < 2 {
			HandleError(fmt.Errorf("invalid environment variable format, expected 'FOO=bar' fromat, got: %v", l))
		}
		wenv[split[0]] = split[1]
	}

	wenv[jwt.NerdTokenEnvVar] = workerJWT.Token
	wenv[jwt.NerdSecretEnvVar] = workerJWT.Secret

	worker, err := bclient.StartWorker(ss.Project.Name, args[0], wenv)
	if err != nil {
		HandleError(err)
	}

	logrus.Infof("Worker Started: %v", worker)
	return nil
}
