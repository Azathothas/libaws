package cliaws

import (
	"context"
	"github.com/alexflint/go-arg"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/nathants/cli-aws/lib"
)

func init() {
	lib.Commands["ec2-rm"] = ec2Rm
	lib.Args["ec2-rm"] = ec2RmArgs{}
}

type ec2RmArgs struct {
	Selectors []string `arg:"positional" help:"instance-ids | dns-names | private-dns-names | tags | vpc-id | subnet-id | security-group-id | ip-addresses | private-ip-addresses"`
	Yes       bool     `arg:"-y,--yes" default:"false"`
}

func (ec2RmArgs) Description() string {
	return "\ndelete an ami\n"
}

func ec2Rm() {
	var args ec2RmArgs
	arg.MustParse(&args)
	ctx := context.Background()
	instances, err := lib.EC2ListInstances(ctx, args.Selectors, "")
	if err != nil {
		lib.Logger.Fatal("error: ", err)
	}
	var ids []*string
	for _, instance := range instances {
		ids = append(ids, instance.InstanceId)
		if *instance.State.Name == ec2.InstanceStateNameRunning || *instance.State.Name == ec2.InstanceStateNameStopped {
			lib.Logger.Println("going to terminate:", lib.EC2Name(instance.Tags), *instance.InstanceId)
		}
	}
	if !args.Yes {
		err = lib.PromptProceed("")
		if err != nil {
			lib.Logger.Fatal("error: ", err)
		}
	}
	_, err = lib.EC2Client().TerminateInstancesWithContext(ctx, &ec2.TerminateInstancesInput{
		InstanceIds: ids,
	})
	if err != nil {
		lib.Logger.Fatal("error: ", err)
	}
}