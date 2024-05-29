package cliaws

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/alexflint/go-arg"
	"github.com/nathants/libaws/lib"
	"os"
)

func init() {
	lib.Commands["ec2-ls"] = ec2Ls
	lib.Args["ec2-ls"] = ec2LsArgs{}
}

type ec2LsArgs struct {
	Selectors []string `arg:"positional" help:"instance-id | dns-name | private-dns-name | tag | vpc-id | subnet-id | security-group-id | ip-address | private-ip-address"`
	State     string   `arg:"-s,--state" default:"" help:"running | pending | terminated | stopped"`
	Dns       bool     `arg:"-d, --dns" help:"include public dns"`
	PrivateIP bool     `arg:"-p, --private-ip" help:"include private ipv4"`
	JSON      bool     `arg:"-j, --json" help:"output in JSON format"`
}

func (ec2LsArgs) Description() string {
	return "\nlist ec2 instances\n"
}

func ec2Ls() {
	var args ec2LsArgs
	arg.MustParse(&args)
	ctx := context.Background()
	instances, err := lib.EC2ListInstances(ctx, args.Selectors, args.State)
	if err != nil {
		lib.Logger.Fatal("error: ", err)
	}

	if args.JSON {
		outputJSON(instances)
		return
	}

	fmt.Fprintln(os.Stderr, "name", "type", "state", "id", "image", "kind", "security-group", "tags")
	count := 0
	for _, instance := range instances {
		count++
		subnet := "-"
		if instance.SubnetId != nil {
			subnet = *instance.SubnetId
		}
		dns := "-"
		if instance.PublicIpAddress != nil {
			dns = *instance.PublicDnsName
		}
		if args.Dns {
			subnet += " " + dns
		}
		ip := "-"
		if instance.PrivateIpAddress != nil {
			ip = *instance.PrivateIpAddress
		}
		if args.PrivateIP {
			subnet += " " + ip
		}

		fmt.Println(
			lib.EC2NameColored(instance),
			*instance.InstanceType,
			*instance.State.Name,
			*instance.InstanceId,
			*instance.ImageId,
			lib.EC2Kind(instance),
			subnet,
			lib.EC2SecurityGroups(instance.SecurityGroups),
			lib.EC2Tags(instance.Tags),
		)
	}
	if count == 0 {
		os.Exit(1)
	}
}

func outputJSON(instances []*lib.EC2Instance) {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(instances); err != nil {
		fmt.Fprintf(os.Stderr, "error encoding JSON: %v\n", err)
		os.Exit(1)
	}
}