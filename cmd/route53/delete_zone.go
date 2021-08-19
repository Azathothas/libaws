package cliaws

import (
	"context"
	"github.com/alexflint/go-arg"
	"github.com/nathants/cli-aws/lib"
)

func init() {
	lib.Commands["route53-delete-zone"] = route53DeleteZone
}

type route53DeleteZoneArgs struct {
	Name    string `arg:"positional,required"`
	Preview bool   `arg:"-p,--preview"`
}

func (route53DeleteZoneArgs) Description() string {
	return "\ndelete hosted zone\n"
}

func route53DeleteZone() {
	var args route53DeleteZoneArgs
	arg.MustParse(&args)
	ctx := context.Background()
	err := lib.Route53DeleteZone(ctx, args.Name, args.Preview)
	if err != nil {
		lib.Logger.Fatal("error: ", err)
	}
}
