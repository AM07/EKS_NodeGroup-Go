package main

import (

	"context"
	"fmt"
	"log"
	"time"

	//AWS Packages ----
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/eks"
	ekstypes "github.com/aws/aws-sdk-go-v2/service/eks/types"
	"github.com/aws/aws-sdk-go-v2/service/sts"

)

func main() {
	var profile string
	var region string
	var myClusterName string
	var newnodegroup string
	var newami string

	fmt.Print("Enter Tenant account profile: ")
	fmt.Scanf("%s\n", &profile)
	fmt.Print("Enter Tenant region: ")
	fmt.Scanf("%s\n", &region)
	fmt.Print("Enter cluster name: ")
	fmt.Scanf("%s\n", &myClusterName)
	fmt.Print("Enter new node group name: ")
	fmt.Scanf("%s\n", &newnodegroup)
	fmt.Print("Enter AMI ID: ")
	fmt.Scanf("%s\n", &newami)

	ctx := context.TODO()
	cfg, err := config.LoadDefaultConfig(
		ctx,
		config.WithRegion(region),
		config.WithSharedConfigProfile(profile),
	)
	if err != nil {
		panic(fmt.Sprintf("Failed loading config, %v", err))
	}

	// Create client connections

	stsclient := sts.NewFromConfig(cfg)
	eksclient := eks.NewFromConfig(cfg)
	ec2client := ec2.NewFromConfig(cfg)

	// Account Details

	identity, err := stsclient.GetCallerIdentity(ctx, &sts.GetCallerIdentityInput{})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("\nWorking on Account: %s, Region: %s\n", aws.ToString(identity.Account), region)

	// Fetch Existing Nodegroups

	input := &eks.ListNodegroupsInput{
		ClusterName: &myClusterName,
	}
	nodegroups, err := eksclient.ListNodegroups(ctx, input)
	if err != nil {
		log.Fatal(err)
	}

	// Fetch LaunchTemplateID from above nodegroups[0]

	input2 := &eks.DescribeNodegroupInput{
		ClusterName:   &myClusterName,
		NodegroupName: &nodegroups.Nodegroups[0],
	}
	nodegroupsdetails, err := eksclient.DescribeNodegroup(ctx, input2)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("\nFirst Nodegroup \"%s\" uses launchTemplate: \"%s\"\n", nodegroups.Nodegroups[0], *nodegroupsdetails.Nodegroup.LaunchTemplate.Id)

	// Get launchTemplate data from above nodegroup[0]

	input3 := &ec2.DescribeLaunchTemplateVersionsInput{
		LaunchTemplateId: nodegroupsdetails.Nodegroup.LaunchTemplate.Id,
		Versions:         []string{"$Latest"},
	}
	launchtemplatedetails, err := ec2client.DescribeLaunchTemplateVersions(ctx, input3)
	if err != nil {
		log.Fatal(err)
	}

	// Data from LaunchTemplate
	// old_ami := *launchtemplatedetails.LaunchTemplateVersions[0].LaunchTemplateData.ImageId
	instanceType := launchtemplatedetails.LaunchTemplateVersions[0].LaunchTemplateData.InstanceType
	securityGroupIds := launchtemplatedetails.LaunchTemplateVersions[0].LaunchTemplateData.SecurityGroupIds
	userData := launchtemplatedetails.LaunchTemplateVersions[0].LaunchTemplateData.UserData
	tags := launchtemplatedetails.LaunchTemplateVersions[0].LaunchTemplateData.TagSpecifications[0].Tags

	// Create LaunchTemplate from above data
	input4 := &ec2.CreateLaunchTemplateInput{
		LaunchTemplateData: &types.RequestLaunchTemplateData{
			InstanceType:     instanceType,
			ImageId:          &newami,
			SecurityGroupIds: securityGroupIds,
			UserData:         userData,
			// IamInstanceProfile: &types.LaunchTemplateIamInstanceProfileSpecificationRequest{
			// 	Arn: iamProfile,
			// },
			TagSpecifications: []types.LaunchTemplateTagSpecificationRequest{
				{
					ResourceType: types.ResourceTypeInstance,
					Tags:         tags,
				},
				{
					ResourceType: types.ResourceTypeNetworkInterface,
					Tags:         tags,
				},
				{
					ResourceType: types.ResourceTypeVolume,
					Tags:         tags,
				}},
		},
		LaunchTemplateName: aws.String(myClusterName + newnodegroup),
		// TagSpecifications: []types.TagSpecification{
		// 	{
		// 		Tags: tags,
		// 	},
		// },
	}
	newtemplate, err := ec2client.CreateLaunchTemplate(ctx, input4)
	if err != nil {
		log.Fatal(err)
	}

	// Create Nodegroup

	// Data from first node group details
	nodeRole := *nodegroupsdetails.Nodegroup.NodeRole
	ngSubnets := nodegroupsdetails.Nodegroup.Subnets
	launchTemplateID := *newtemplate.LaunchTemplate.LaunchTemplateId
	// scaleConfig := *&nodegroupsdetails.Nodegroup.ScalingConfig

	input5 := &eks.CreateNodegroupInput{
		ClusterName:   &myClusterName,
		NodeRole:      &nodeRole,
		NodegroupName: &newnodegroup,
		Subnets:       ngSubnets,
		// ScalingConfig: scaleConfig,
		ScalingConfig: &ekstypes.NodegroupScalingConfig{
			DesiredSize: aws.Int32(2),
			MaxSize:     aws.Int32(2),
			MinSize:     aws.Int32(2),
		},
		LaunchTemplate: &ekstypes.LaunchTemplateSpecification{
			Id: &launchTemplateID,
		},
		Tags: map[string]string{
			"Project":         "AMS",
			"SecurityZone":    "T",
			"TaggingVersion":  "V2.4",
			"Confidentiality": "C3",
			"Environment":     "TEST",
			"ManagedBy":       "AMS",
		},
	}

	newnodegroupout, err := eksclient.CreateNodegroup(ctx, input5)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("\nNew Nodegroup Name: \n\"%v\"\n", *newnodegroupout.Nodegroup.NodegroupName)

	// Calling Sleep method
	time.Sleep(10 * time.Second)

	// Printed after sleep is over
	fmt.Println("Sleep Over.....")
}