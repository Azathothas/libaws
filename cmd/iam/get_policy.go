package cliaws

import (
	"context"
	"fmt"
	"strings"

	"github.com/alexflint/go-arg"
	"github.com/aws/aws-sdk-go/service/iam"
	"github.com/nathants/libaws/lib"
)

func init() {
	lib.Commands["iam-get-policy"] = iamGetPolicy
	lib.Args["iam-get-policy"] = iamGetPolicyArgs{}
}

type iamGetPolicyArgs struct {
	Name string `arg:"positional,required"`
}

func (iamGetPolicyArgs) Description() string {
	return "\nget iam policy\n"
}

func iamGetPolicy() {
	var args iamGetPolicyArgs
	arg.MustParse(&args)
	ctx := context.Background()
	policies, err := lib.IamListPolicies(ctx)
	if err != nil {
		lib.Logger.Fatal("error: ", err)
	}
	var matches []*iam.Policy
	for _, policy := range policies {
		if strings.EqualFold(lib.Last(strings.Split(*policy.Arn, "/")), args.Name) {
			matches = append(matches, policy)
		}
	}
	switch len(matches) {
	case 0:
		lib.Logger.Fatal("error: ", "no policy found")
	case 1:
		p := &lib.IamPolicy{}
		err := p.FromPolicy(ctx, matches[0], true)
		if err != nil {
			lib.Logger.Fatal("error: ", err)
		}
		fmt.Println(lib.Pformat(p))
	default:
		lib.Logger.Fatal("error: ", "more than 1 policy found:", lib.Pformat(matches))
	}
}
