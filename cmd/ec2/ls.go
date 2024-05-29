package cliaws

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/alexflint/go-arg"
	"github.com/nathants/libaws/lib"
	"os"
	"regexp"
)

//const ansi = "[\u001B\u009B][[\\]()#;?]*(?:(?:(?:[a-zA-Z\\d]*(?:;[a-zA-Z\\d]*)*)?\u0007)|(?:(?:\\d{1,4}(?:;\\d{0,4})*)?[\\dA-PRZcf-ntqry=><~]))"
const ansi = "(?i)[\u001B\u009B][[\\]()#;?]*(?:(?:(?:[a-zA-Z\\d]*(?:;[a-zA-Z\\d]*)*)?\u0007)|(?:(?:\\d{1,4}(?:;\\d{0,4})*)?[\\dA-PRZcf-ntqry=><~]))"
var re = regexp.MustCompile(ansi)
func Strip(str string) string {
	return re.ReplaceAllString(str, "")
}

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

type instanceOutput struct {
	Name          string `json:"name"`
	Type          string `json:"type"`
	State         string `json:"state"`
	ID            string `json:"id"`
	Image         string `json:"image"`
	Kind          string `json:"kind"`
	SecurityGroup string `json:"security-group"`
	Tags          string `json:"tags"`
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
		// Output in JSON format
		var output []instanceOutput
		for _, instance := range instances {
			output = append(output, instanceOutput{
				Name:          Strip(lib.EC2NameColored(instance)),
                                Type:          Strip(*instance.InstanceType),
                                State:         Strip(*instance.State.Name),
                                ID:            Strip(*instance.InstanceId),
                                Image:         Strip(*instance.ImageId),
                                Kind:          Strip(lib.EC2Kind(instance)),
                                SecurityGroup: Strip(lib.EC2SecurityGroups(instance.SecurityGroups)),
                                Tags:          Strip(lib.EC2Tags(instance.Tags)),
			})
		}
		jsonOutput, err := json.MarshalIndent(output, "", "  ")
		if err != nil {
			lib.Logger.Fatal("error marshaling JSON: ", err)
		}
		fmt.Println(string(jsonOutput))
	} else {
		// Output in tabular format
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
				Strip(lib.EC2NameColored(instance)),
				*instance.InstanceType,
				*instance.State.Name,
				*instance.InstanceId,
				*instance.ImageId,
				Strip(lib.EC2Kind(instance)),
				subnet,
				Strip(lib.EC2SecurityGroups(instance.SecurityGroups)),
				Strip(lib.EC2Tags(instance.Tags)),
			)
		}
		if count == 0 {
			os.Exit(1)
		}
	}
}
