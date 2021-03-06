package command

import (
	"fmt"
	"strconv"

	"github.com/Sirupsen/logrus"
	"github.com/mitchellh/cli"
	"github.com/pkg/errors"
)

//TaskDescribe command
type TaskDescribe struct {
	*command
}

//TaskDescribeFactory returns a factory method for the join command
func TaskDescribeFactory() (cli.Command, error) {
	comm, err := newCommand("nerd task describe <queue-id> <task-id>", "return more information about a specific task", "", nil)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create command")
	}
	cmd := &TaskDescribe{
		command: comm,
	}
	cmd.runFunc = cmd.DoRun

	return cmd, nil
}

//DoRun is called by run and allows an error to be returned
func (cmd *TaskDescribe) DoRun(args []string) (err error) {
	if len(args) < 2 {
		return fmt.Errorf("not enough arguments, see --help")
	}

	bclient, err := NewClient(cmd.ui, cmd.config, cmd.session)
	if err != nil {
		HandleError(err)
	}

	taskID, err := strconv.ParseInt(args[1], 10, 64)
	if err != nil {
		HandleError(errors.Wrap(err, "invalid task ID, must be a number"))
	}

	ss, err := cmd.session.Read()
	if err != nil {
		HandleError(err)
	}
	out, err := bclient.DescribeTask(ss.Project.Name, args[0], taskID)
	if err != nil {
		HandleError(err)
	}

	logrus.Infof("Task Description: %+v", out)
	return nil
}
