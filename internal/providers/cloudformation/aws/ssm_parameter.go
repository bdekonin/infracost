package aws

import (
	"github.com/awslabs/goformation/v7/cloudformation/ssm"
	"github.com/rs/zerolog/log"

	"github.com/infracost/infracost/internal/resources/aws"
	"github.com/infracost/infracost/internal/schema"
)

func GetSSMParameterRegistryItem() *schema.RegistryItem {
	return &schema.RegistryItem{
		Name: "AWS::SSM::Parameter",
		Notes: []string{
			"SSM Parameters are not yet supported.",
		},
		RFunc: NewSSMParameter,
	}
}

func NewSSMParameter(d *schema.ResourceData, u *schema.UsageData) *schema.Resource {
	cfr, ok := d.CFResource.(*ssm.Parameter)
	if !ok {
		log.Warn().Msgf("Skipping resource %s as it did not have the expected type (got %T)", d.Address, d.CFResource)
		return nil
	}

	region := "us-east-1" // TODO figure out how to set region

	if cfr.Tier == nil {
		tier := "Standard"
		cfr.Tier = &tier
	}

	a := &aws.SSMParameter{
		Address: d.Address,
		Region: region,
		Tier: *cfr.Tier,
	}
	a.PopulateUsage(u)
	
	resource := a.BuildResource()

	return resource
}
