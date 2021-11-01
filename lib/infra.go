package lib

import (
	"context"
	"fmt"
	"reflect"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/apigateway"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/lambda"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/sqs"
)

type InfraApi struct{}

type InfraDynamoDB struct {
	Keys  []string `json:"keys,omitempty"`
	Attrs []string `json:"attrs,omitempty"`
}

type InfraEC2 struct {
	Attrs []string `json:"attrs,omitempty"`
}

type InfraLambda struct {
	Policies []string `json:"policies,omitempty"`
	Allows   []string `json:"allows,omitempty"`
	Triggers []string `json:"triggers,omitempty"`
	Attrs    []string `json:"attrs,omitempty"`
}

type InfraSQS struct {
	Attrs []string `json:"attrs,omitempty"`
}

type InfraS3 struct {
	Attrs []string `json:"attrs,omitempty"`
}

type Infra struct {
	Account  string                   `json:"account"`
	Api      map[string]InfraApi      `json:"api,omitempty"`
	DynamoDB map[string]InfraDynamoDB `json:"dynamodb,omitempty"`
	EC2      map[string]InfraEC2      `json:"ec2,omitempty"`
	Lambda   map[string]InfraLambda   `json:"lambda,omitempty"`
	SQS      map[string]InfraSQS      `json:"sqs,omitempty"`
	S3       map[string]InfraS3       `json:"s3,omitempty"`
}

type InfraLambdaTrigger struct {
	LambdaName   string
	TriggerType  string
	TriggerAttrs []string
}

func InfraList(ctx context.Context, filter string) (*Infra, error) {
	var err error
	infra := &Infra{}
	account, err := StsAccount(ctx)
	if err != nil {
		Logger.Fatal("error: ", err)
	}
	infra.Account = account

	errs := make(chan error)
	count := 0
	triggersChan := make(chan InfraLambdaTrigger, 1024)

	run := func(fn func()) {
		go fn()
		count++
	}

	run(func() {
		infra.Api, err = InfraListApi(ctx, triggersChan)
		errs <- err
	})

	run(func() {
		infra.DynamoDB, err = InfraListDynamoDB(ctx)
		errs <- err
	})

	run(func() {
		infra.EC2, err = InfraListEC2(ctx)
		errs <- err
	})

	run(func() {
		infra.SQS, err = InfraListSQS(ctx)
		errs <- err
	})

	run(func() {
		infra.S3, err = InfraListS3(ctx, triggersChan)
		errs <- err
	})

	run(func() {
		_, err = InfraListCloudwatch(ctx, triggersChan)
		errs <- err
	})

	lambdaErr := make(chan error)
	go func() {
		infra.Lambda, err = InfraListLambda(ctx, triggersChan, filter)
		lambdaErr <- err
	}()

	for i := 0; i < count; i++ {
		err := <-errs
		if err != nil {
			Logger.Fatal("error: ", err)
		}
	}
	close(triggersChan)

	err = <-lambdaErr
	if err != nil {
		Logger.Fatal("error: ", err)
	}

	return infra, nil
}

func InfraListCloudwatch(ctx context.Context, triggersChan chan<- InfraLambdaTrigger) (map[string]string, error) {
	Logger.Println("cloudwatch list rules")
	rules, err := EventsListRules(ctx)
	if err != nil {
		Logger.Println("error:", err)
		return nil, err
	}
	for _, rule := range rules {
		Logger.Println("cloudwatch list targets for for rule:", *rule.Name)
		targets, err := EventsListRuleTargets(ctx, *rule.Name)
		if err != nil {
			Logger.Println("error:", err)
			return nil, err
		}
		for _, target := range targets {
			if strings.HasPrefix(*target.Arn, "arn:aws:lambda:") {
				triggersChan <- InfraLambdaTrigger{
					LambdaName:   Last(strings.Split(*target.Arn, ":")),
					TriggerType:  lambdaTriggerCloudwatch,
					TriggerAttrs: []string{*rule.ScheduleExpression},
				}
			}
		}
	}
	return nil, nil
}

func InfraListLambda(ctx context.Context, triggersChan <-chan InfraLambdaTrigger, filter string) (map[string]InfraLambda, error) {
	Logger.Println("lambda list functions")
	allFns, err := LambdaListFunctions(ctx)
	if err != nil {
		Logger.Println("error:", err)
		return nil, err
	}
	var fns []*lambda.FunctionConfiguration
	for _, fn := range allFns {
		if filter != "" && !strings.Contains(*fn.FunctionName, filter) {
			continue
		}
		fns = append(fns, fn)
	}
	triggers := make(map[string][]InfraLambdaTrigger)
	res := make(map[string]InfraLambda)
	for _, fn := range fns {
		l := InfraLambda{}
		if *fn.MemorySize != 128 { // default
			l.Attrs = append(l.Attrs, fmt.Sprintf("memory %d", *fn.MemorySize))
		}
		if *fn.Timeout != 3 { // default
			l.Attrs = append(l.Attrs, fmt.Sprintf("timeout %d", *fn.Timeout))
		}
		//
		Logger.Println("lambda get concurrency for:", *fn.FunctionName)
		out, err := LambdaClient().GetFunctionConcurrencyWithContext(ctx, &lambda.GetFunctionConcurrencyInput{
			FunctionName: aws.String(*fn.FunctionName),
		})
		if err != nil {
			Logger.Println("error:", err)
			return nil, err
		}
		if out.ReservedConcurrentExecutions != nil {
			l.Attrs = append(l.Attrs, fmt.Sprintf("concurrency %d", *out.ReservedConcurrentExecutions))
		}
		//
		roleName := Last(strings.Split(*fn.Role, "/"))
		//
		Logger.Println("lambda list roles for:", *fn.FunctionName)
		policies, err := IamListRolePolicies(ctx, roleName)
		if err != nil {
			Logger.Println("error:", err)
			return nil, err
		}
		for _, policy := range policies {
			l.Policies = append(l.Policies, *policy.PolicyName)
		}
		//
		Logger.Println("lambda list allows for:", *fn.FunctionName)
		allows, err := IamListRoleAllows(ctx, roleName)
		if err != nil {
			Logger.Println("error:", err)
			return nil, err
		}
		for _, allow := range allows {
			l.Allows = append(l.Allows, allow.String())
		}
		//
		var marker *string
		for {
			out, err := LambdaClient().ListEventSourceMappingsWithContext(ctx, &lambda.ListEventSourceMappingsInput{
				FunctionName: fn.FunctionArn,
				Marker:       marker,
			})
			if err != nil {
				Logger.Println("error:", err)
				return nil, err
			}
			for _, mapping := range out.EventSourceMappings {
				if Contains([]string{"Disabled", "Disabling"}, *mapping.State) {
					continue
				}
				infra := ArnToInfraName(*mapping.EventSourceArn)
				switch infra {
				case lambdaTriggerSQS, lambdaTriggerDynamoDB:
					var sourceName string
					switch infra {
					case lambdaTriggerSQS:
						sourceName = SQSArnToName(*mapping.EventSourceArn)
					case lambdaTriggerDynamoDB:
						sourceName = DynamoDBStreamArnToTableName(*mapping.EventSourceArn)
					default:
						err := fmt.Errorf("unknown infra: %s", infra)
						Logger.Println("error:", err)
						return nil, err
					}

					triggers[*fn.FunctionName] = append(triggers[*fn.FunctionName], InfraLambdaTrigger{
						LambdaName:  *fn.FunctionName,
						TriggerType: *mapping.EventSourceArn,
						TriggerAttrs: []string{
							sourceName,
							fmt.Sprintf("batch=%d", *mapping.BatchSize),
							fmt.Sprintf("parallel=%d", *mapping.ParallelizationFactor),
							fmt.Sprintf("retry=%d", *mapping.MaximumRetryAttempts),
							fmt.Sprintf("start=%s", *mapping.StartingPosition),
							fmt.Sprintf("window=%d", *mapping.MaximumBatchingWindowInSeconds),
						},
					})
				default:
					Logger.Println("ignoring event source mapping:", *mapping.FunctionArn, *mapping.EventSourceArn)
				}
			}
			if out.NextMarker == nil {
				break
			}
			marker = out.NextMarker
		}
		//
		res[*fn.FunctionName] = l
	}
	//
	Logger.Println("lambda wait for triggers")
	for trigger := range triggersChan {
		triggers[trigger.LambdaName] = append(triggers[trigger.LambdaName], trigger)
	}
	Logger.Println("lambda got all triggers")
	for _, fn := range fns {
		ts, ok := triggers[*fn.FunctionName]
		if ok {
			for _, trigger := range ts {
				val := trigger.TriggerType
				if len(trigger.TriggerAttrs) > 0 {
					val += " " + strings.Join(trigger.TriggerAttrs, " ")
				}
				l, ok := res[*fn.FunctionName]
				if !ok {
					panic(*fn.FunctionName)
				}
				l.Triggers = append(l.Triggers, val)
				res[*fn.FunctionName] = l
			}
		}
	}
	//
	return res, nil
}

func InfraListApi(ctx context.Context, triggersChan chan<- InfraLambdaTrigger) (map[string]InfraApi, error) {
	infraApi := make(map[string]InfraApi)
	apis, err := apiList(ctx)
	if err != nil {
		Logger.Println("error:", err)
		return nil, err
	}
	for _, api := range apis {
		infraApi[*api.Name] = InfraApi{}
		parentID, err := ApiResourceID(ctx, *api.Id, "/")
		if err != nil {
			Logger.Println("error:", err)
			return nil, err
		}
		out, err := ApiClient().GetIntegrationWithContext(ctx, &apigateway.GetIntegrationInput{
			RestApiId:  api.Id,
			HttpMethod: aws.String(apiHttpMethod),
			ResourceId: aws.String(parentID),
		})
		if err != nil {
			Logger.Println("error:", err)
			return nil, err
		}
		lambdaName := LambdaApiUriToLambdaName(*out.Uri)
		triggersChan <- InfraLambdaTrigger{
			LambdaName:  lambdaName,
			TriggerType: lambdaTriggerApi,
		}
	}
	return infraApi, nil
}

func InfraListDynamoDB(ctx context.Context) (map[string]InfraDynamoDB, error) {
	infraDynamoDB := make(map[string]InfraDynamoDB)
	tableNames, err := DynamoDBListTables(ctx)
	if err != nil {
		Logger.Println("error:", err)
		return nil, err
	}
	for _, tableName := range tableNames {
		db := InfraDynamoDB{}
		out, err := DynamoDBClient().DescribeTableWithContext(ctx, &dynamodb.DescribeTableInput{
			TableName: aws.String(tableName),
		})
		if err != nil {
			Logger.Fatal("error: ", err)
		}
		attrTypes := make(map[string]string)
		for _, attr := range out.Table.AttributeDefinitions {
			attrTypes[*attr.AttributeName] = *attr.AttributeType
		}
		for _, key := range out.Table.KeySchema {
			db.Keys = append(db.Keys, fmt.Sprintf("%s:%s:%s", *key.AttributeName, attrTypes[*key.AttributeName], *key.KeyType))
		}
		db.Attrs = append(db.Attrs, fmt.Sprintf("ProvisionedThroughput.ReadCapacityUnits=%d", *out.Table.ProvisionedThroughput.ReadCapacityUnits))
		db.Attrs = append(db.Attrs, fmt.Sprintf("ProvisionedThroughput.WriteCapacityUnits=%d", *out.Table.ProvisionedThroughput.WriteCapacityUnits))
		if out.Table.StreamSpecification != nil {
			db.Attrs = append(db.Attrs, fmt.Sprintf("StreamSpecification.StreamViewType=%s", *out.Table.StreamSpecification.StreamViewType))
		}
		for i, index := range out.Table.LocalSecondaryIndexes {
			db.Attrs = append(db.Attrs, fmt.Sprintf("LocalSecondaryIndexes.%d.IndexName=%s", i, *index.IndexName))
			for j, key := range index.KeySchema {
				db.Attrs = append(db.Attrs, fmt.Sprintf("LocalSecondaryIndexes.%d.Key.%d=%s:%s:%s", i, j, *key.AttributeName, attrTypes[*key.AttributeName], *key.KeyType))
			}
			db.Attrs = append(db.Attrs, fmt.Sprintf("LocalSecondaryIndexes.%d.Projection.ProjectionType=%s", i, *index.Projection.ProjectionType))
			for j, attr := range index.Projection.NonKeyAttributes {
				db.Attrs = append(db.Attrs, fmt.Sprintf("LocalSecondaryIndexes.%d.Projection.NonKeyAttributes.%d=%s", i, j, *attr))
			}
		}
		for i, index := range out.Table.GlobalSecondaryIndexes {
			db.Attrs = append(db.Attrs, fmt.Sprintf("GlobalSecondaryIndexes.%d.IndexName=%s", i, *index.IndexName))
			for j, key := range index.KeySchema {
				db.Attrs = append(db.Attrs, fmt.Sprintf("GlobalSecondaryIndexes.%d.Key.%d=%s:%s:%s", i, j, *key.AttributeName, attrTypes[*key.AttributeName], *key.KeyType))
			}
			db.Attrs = append(db.Attrs, fmt.Sprintf("GlobalSecondaryIndexes.%d.Projection.ProjectionType=%s", i, *index.Projection.ProjectionType))
			for j, attr := range index.Projection.NonKeyAttributes {
				db.Attrs = append(db.Attrs, fmt.Sprintf("GlobalSecondaryIndexes.%d.Projection.NonKeyAttributes.%d=%s", i, j, *attr))
			}
			db.Attrs = append(db.Attrs, fmt.Sprintf("GlobalSecondaryIndexes.%d.ProvisionedThroughput.ReadCapacityUnits=%d", i, *index.ProvisionedThroughput.ReadCapacityUnits))
			db.Attrs = append(db.Attrs, fmt.Sprintf("GlobalSecondaryIndexes.%d.ProvisionedThroughput.WriteCapacityUnits=%d", i, *index.ProvisionedThroughput.WriteCapacityUnits))
		}
		tags, err := DynamoDBListTags(ctx, tableName)
		if err != nil {
			Logger.Println("error:", err)
			return nil, err
		}
		for i, tag := range tags {
			db.Attrs = append(db.Attrs, fmt.Sprintf("Tags.%d.Key=%s", i, *tag.Key))
			db.Attrs = append(db.Attrs, fmt.Sprintf("Tags.%d.Value=%s", i, *tag.Value))
		}
		infraDynamoDB[tableName] = db
	}
	return infraDynamoDB, nil
}

func InfraListEC2(ctx context.Context) (map[string]InfraEC2, error) {
	infraEC2 := make(map[string]InfraEC2)
	instances, err := EC2ListInstances(ctx, nil, "running")
	if err != nil {
		Logger.Println("error:", err)
		return nil, err
	}
	for _, instance := range instances {
		ec2 := InfraEC2{}
		ec2.Attrs = append(ec2.Attrs, fmt.Sprintf("Type=%s", *instance.InstanceType))
		ec2.Attrs = append(ec2.Attrs, fmt.Sprintf("Image=%s", *instance.ImageId))
		ec2.Attrs = append(ec2.Attrs, fmt.Sprintf("Kind=%s", EC2Kind(instance)))
		ec2.Attrs = append(ec2.Attrs, fmt.Sprintf("Vpc=%s", *instance.VpcId))
		for _, tag := range instance.Tags {
			if *tag.Key != "creation-date" && *tag.Key != "Name" {
				ec2.Attrs = append(ec2.Attrs, fmt.Sprintf("Tags.%s=%s", *tag.Key, *tag.Value))
			}
		}
		infraEC2[EC2Name(instance.Tags)] = ec2
	}
	return infraEC2, nil
}

func InfraListS3(ctx context.Context, triggersChan chan<- InfraLambdaTrigger) (map[string]InfraS3, error) {
	res := make(map[string]InfraS3)
	buckets, err := S3Client().ListBucketsWithContext(ctx, &s3.ListBucketsInput{})
	if err != nil {
		Logger.Println("error:", err)
		return nil, err
	}
	for _, bucket := range buckets.Buckets {
		s := InfraS3{}
		descr, err := S3GetBucketDescription(ctx, *bucket.Name)
		if err != nil {
			Logger.Println("error:", err)
			return nil, err
		}
		s3Default := s3EnsureInputDefault()
		//
		if descr.Policy == nil && s3Default.acl != "private" {
			s.Attrs = append(s.Attrs, "acl=private")
		} else if descr.Policy != nil && reflect.DeepEqual(s3PublicPolicy(*bucket.Name), *descr.Policy) && s3Default.acl != "public" {
			s.Attrs = append(s.Attrs, "acl=public")
		}
		//
		if descr.Versioning != s3Default.versioning {
			s.Attrs = append(s.Attrs, fmt.Sprintf("versioning=%t", descr.Versioning))
		}
		//
		encryption := reflect.DeepEqual(descr.Encryption, s3EncryptionConfig)
		if encryption != s3Default.encryption {
			s.Attrs = append(s.Attrs, fmt.Sprintf("encryption=%t", encryption))
		}
		//
		metrics := descr.Metrics != nil
		if s3Default.metrics != metrics {
			s.Attrs = append(s.Attrs, fmt.Sprintf("metrics=%t", metrics))
		}
		//
		if descr.Notifications != nil {
			for _, conf := range descr.Notifications.LambdaFunctionConfigurations {
				triggersChan <- InfraLambdaTrigger{
					LambdaName:   LambdaArnToLambdaName(*conf.LambdaFunctionArn),
					TriggerType:  lambdaTrigerS3,
					TriggerAttrs: []string{*bucket.Name},
				}
			}
		}
		res[*bucket.Name] = s
	}
	return res, nil
}

func InfraListSQS(ctx context.Context) (map[string]InfraSQS, error) {
	urls, err := SQSListQueueUrls(ctx)
	if err != nil {
		Logger.Println("error:", err)
		return nil, err
	}
	res := make(map[string]InfraSQS)
	for _, url := range urls {
		if err != nil {
			Logger.Println("error:", err)
			return nil, err
		}
		out, err := SQSClient().GetQueueAttributesWithContext(ctx, &sqs.GetQueueAttributesInput{
			QueueUrl: aws.String(url),
			AttributeNames: []*string{
				aws.String("DelaySeconds"),
				aws.String("MaximumMessageSize"),
				aws.String("MessageRetentionPeriod"),
				aws.String("ReceiveMessageWaitTimeSeconds"),
				aws.String("VisibilityTimeout"),
				aws.String("KmsDataKeyReusePeriodSeconds"),
			},
		})
		if err != nil {
			Logger.Println("error:", err)
			return nil, err
		}
		s := InfraSQS{}
		if out.Attributes["DelaySeconds"] != nil && *out.Attributes["DelaySeconds"] != "0" { // default
			s.Attrs = append(s.Attrs, "DelaySeconds="+*out.Attributes["DelaySeconds"])
		}
		if out.Attributes["MaximumMessageSize"] != nil && *out.Attributes["MaximumMessageSize"] != "262144" { // default
			s.Attrs = append(s.Attrs, "MaximumMessageSize="+*out.Attributes["MaximumMessageSize"])
		}
		if out.Attributes["MessageRetentionPeriod"] != nil && *out.Attributes["MessageRetentionPeriod"] != "345600" { // default
			s.Attrs = append(s.Attrs, "MessageRetentionPeriod="+*out.Attributes["MessageRetentionPeriod"])
		}
		if out.Attributes["ReceiveMessageWaitTimeSeconds"] != nil && *out.Attributes["ReceiveMessageWaitTimeSeconds"] != "0" { // default
			s.Attrs = append(s.Attrs, "ReceiveMessageWaitTimeSeconds="+*out.Attributes["ReceiveMessageWaitTimeSeconds"])
		}
		if out.Attributes["VisibilityTimeout"] != nil && *out.Attributes["VisibilityTimeout"] != "30" { // default
			s.Attrs = append(s.Attrs, "VisibilityTimeout="+*out.Attributes["VisibilityTimeout"])
		}
		if out.Attributes["KmsDataKeyReusePeriodSeconds"] != nil && *out.Attributes["KmsDataKeyReusePeriodSeconds"] != "300" { // default
			s.Attrs = append(s.Attrs, "KmsDataKeyReusePeriodSeconds="+*out.Attributes["KmsDataKeyReusePeriodSeconds"])
		}
		res[SQSUrlToName(url)] = s
	}
	return res, nil
}

func InfraEnsureS3(ctx context.Context, buckets []string, preview bool) error {
	for _, bucket := range buckets {
		parts := strings.Split(bucket, " ")
		name := parts[0]
		attrs := parts[1:]
		input, err := S3EnsureInput(name, attrs)
		if err != nil {
			Logger.Println("error:", err)
			return err
		}
		err = S3Ensure(ctx, input, preview)
		if err != nil {
			Logger.Println("error:", err)
			return err
		}
	}
	return nil
}

func InfraEnsureDynamoDB(ctx context.Context, dbs []string, preview bool) error {
	for _, db := range dbs {
		parts := strings.Split(db, " ")
		name := parts[0]
		var keys []string
		var attrs []string
		for _, part := range parts[1:] {
			if strings.Contains(part, "=") {
				attrs = append(attrs, part)
			} else {
				keys = append(keys, part)
			}
		}
		input, err := DynamoDBEnsureInput(name, keys, attrs)
		if err != nil {
			Logger.Println("error:", err)
			return err
		}
		err = DynamoDBEnsure(ctx, input, preview)
		if err != nil {
			Logger.Println("error:", err)
			return err
		}
	}
	return nil
}

func InfraEnsureSqs(ctx context.Context, queues []string, preview bool) error {
	for _, queue := range queues {
		parts := strings.Split(queue, "/")
		name := parts[0]
		attrs := parts[1:]
		input, err := SQSEnsureInput(name, attrs)
		if err != nil {
			Logger.Println("error:", err)
			return err
		}
		err = SQSEnsure(ctx, input, preview)
		if err != nil {
			Logger.Println("error:", err)
			return err
		}
	}
	return nil
}
