package command

import (
	"io"
	"net/url"
	"strings"

	pb "gopkg.in/cheggaaa/pb.v1"

	"github.com/Sirupsen/logrus"
	"github.com/mitchellh/cli"
	v1auth "github.com/nerdalize/nerd/nerd/client/auth/v1"
	v1batch "github.com/nerdalize/nerd/nerd/client/batch/v1"
	"github.com/nerdalize/nerd/nerd/conf"
	"github.com/nerdalize/nerd/nerd/jwt"
	"github.com/pkg/errors"
	"github.com/restic/chunker"
)

type stdoutkw struct{}

//Write writes a key to stdout.
func (kw *stdoutkw) Write(k string) (err error) {
	// _, err = fmt.Fprintf(os.Stdout, "%v\n", k)
	logrus.Info(k)
	return nil
}

//NewClient creates a new NerdAPIClient with two credential providers.
func NewClient(ui cli.Ui) (*v1batch.Client, error) {
	c, err := conf.Read()
	if err != nil {
		return nil, errors.Wrap(err, "failed to read config")
	}
	key, err := jwt.ParseECDSAPublicKeyFromPemBytes([]byte(c.Auth.PublicKey))
	if err != nil {
		return nil, errors.Wrap(err, "ECDSA Public Key is invalid")
	}
	base, err := url.Parse(c.NerdAPIEndpoint)
	if err != nil {
		return nil, errors.Wrapf(err, "nerd endpoint '%v' is not a valid URL", c.NerdAPIEndpoint)
	}
	return v1batch.NewClient(v1batch.ClientConfig{
		JWTProvider: v1batch.NewChainedJWTProvider(
			jwt.NewEnvProvider(key),
			jwt.NewConfigProvider(key),
			jwt.NewAuthAPIProvider(key, UserPassProvider(ui), v1auth.NewClient(c.Auth.APIEndpoint)),
		),
		Base:   base,
		Logger: logrus.StandardLogger(),
	}), nil
}

//UserPassProvider prompts the username and password on stdin.
func UserPassProvider(ui cli.Ui) func() (string, string, error) {
	return func() (string, string, error) {
		ui.Info("Please enter your Nerdalize username and password.")
		user, err := ui.Ask("Username: ")
		if err != nil {
			return "", "", errors.Wrap(err, "failed to read username")
		}
		pass, err := ui.AskSecret("Password: ")
		if err != nil {
			return "", "", errors.Wrap(err, "failed to read password")
		}
		return user, pass, nil
	}
}

//ErrorCauser returns the error that is one level up in the error chain.
func ErrorCauser(err error) error {
	type causer interface {
		Cause() error
	}

	if err2, ok := err.(causer); ok {
		err = err2.Cause()
	}
	return err
}

//printUserFacing will try to get the user facing error message from the error chain and print it.
func printUserFacing(err error, verbose bool) {
	cause := errors.Cause(err)
	type userFacing interface {
		UserFacingMsg() string
		Underlying() error
	}
	if uerr, ok := cause.(userFacing); ok {
		logrus.Info(uerr.UserFacingMsg())
		logrus.Debugf("Underlying error: %v", uerr.Underlying())
		logrus.Exit(-1)
	}
}

//HandleError handles the way errors are presented to the user.
func HandleError(err error, verbose bool) {
	printUserFacing(err, verbose)
	// when there's are more than 1 message on the message stack, only print the top one for user friendlyness.
	if errors.Cause(err) != nil {
		logrus.Info(strings.Replace(err.Error(), ": "+ErrorCauser(ErrorCauser(err)).Error(), "", 1))
	}
	logrus.Debugf("Underlying error: %+v", err)
	logrus.Exit(-1)
}

//ProgressBar creates a new CLI progess bar and adds input from the progressCh to the bar.
func ProgressBar(total int64, progressCh <-chan int64, doneCh chan<- struct{}) {
	bar := pb.New64(total).Start()
	bar.SetUnits(pb.U_BYTES)
	for elem := range progressCh {
		bar.Add64(elem)
	}
	bar.Finish()
	doneCh <- struct{}{}
}

type Chunker struct {
	cr *chunker.Chunker
}

func NewChunker(pol uint64, r io.Reader) *Chunker {
	return &Chunker{
		cr: chunker.New(r, chunker.Pol(pol)),
	}
}

func (c *Chunker) Next() (data []byte, length uint, err error) {
	buf := make([]byte, chunker.MaxSize)
	chunk, err := c.cr.Next(buf)
	if err != nil {
		return []byte{}, 0, err
	}
	return chunk.Data, chunk.Length, nil
}
