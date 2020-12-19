package ec2

import (
	"context"
	"fmt"
	"github.com/alexflint/go-arg"
	"github.com/nathants/cli-aws/lib"
	"github.com/aws/aws-sdk-go/service/ec2"
)

func init() {
	lib.Commands["ec2-new"] = ec2New
}

type newArgs struct {
	Name      string   `arg:"positional"`
	Num       int      `arg:"-n,--num" default:"1"`
	Type      string   `arg:"-t,--type"`
	Ami       string   `arg:"-a,--ami"`
	Key       string   `arg:"-k,--key"`
	Spot      bool     `arg:"-s,--spot" default:"false"`
	SgID      string   `arg:"--sg"`
	SubnetIds []string `arg:"--subnets"`
	Gigs      int      `arg:"-g,--gigs" default:"16"`
}

func (newArgs) Description() string {
	return "\ncreate ec2 instances\n"
}

func ec2New() {
	var args newArgs
	arg.MustParse(&args)
	ctx, cancel := context.WithCancel(context.Background())
	lib.SignalHandler(cancel)
	var instances []*ec2.Instance
	var err error
	if args.Spot {
		instances, err = lib.RequestSpotFleet(ctx, &lib.FleetConfig{
			NumInstances:  args.Num,
			AmiID:         args.Ami,
			InstanceTypes: []string{args.Type},
			Name:          args.Name,
			Key:           args.Key,
			SgID:          args.SgID,
			SubnetIds:     args.SubnetIds,
			Gigs:          args.Gigs,
		})
	} else {
		panic("todo")
	}
	if err != nil {
		lib.Logger.Fatalf("error: %s\n", err)
	}
	for _, instance := range instances {
		fmt.Println(*instance.InstanceId)
	}

}