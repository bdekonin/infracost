package aws

import (
	// "encoding/json"

	"github.com/awslabs/goformation/v4/cloudformation/apigatewayv2"
	"github.com/rs/zerolog/log"

	"github.com/infracost/infracost/internal/resources/aws"
	"github.com/infracost/infracost/internal/schema"
)

func GetAPIGatewayv2ApiRegistryItem() *schema.RegistryItem {
	return &schema.RegistryItem{
		Name: "AWS::ApiGatewayV2::Api",
		Notes: []string{
			"ApiGatewayV2 Apis are not yet supported.",
		},
		RFunc: NewApiGatewayV2Api,
	}
}

func NewApiGatewayV2Api(d *schema.ResourceData, u *schema.UsageData) *schema.Resource {
	cfr, ok := d.CFResource.(*apigatewayv2.Api)
	if !ok {
		log.Warn().Msgf("Skipping resource %s as it did not have the expected type (got %T)", d.Address, d.CFResource)
		return nil
	}

	region := "us-east-1" // TODO figure out how to set region
	
	// m := make(map[string]interface{})
	// err := json.Unmarshal([]byte(cfr.TaskDefinition), &m)
	// if err != nil {
	// 	panic(err)
	// }
	// // Perform type assertion to convert interface{} to float64
	// memory, _ := m["Memory"].(float64)
	// cpu, _ := m["Cpu"].(float64)

	// var accelerator string
	// if inferenceAccelerator, ok := m["InferenceAccelerator"].(map[string]interface{}); ok {
	// 	if deviceType, ok := inferenceAccelerator["DeviceType"].(string); ok {
	// 		accelerator = deviceType
	// 	}
	// }

	a := &aws.APIGatewayV2API{
		Address:               			d.Address,
		Region:                			region,
		ProtocolType:          			cfr.ProtocolType,
	}
	a.PopulateUsage(u)

	resource := a.BuildResource()
	resource.Tags = mapTags(cfr.Tags)

	return resource
}