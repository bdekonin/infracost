package aws

import (
	// "strconv"
	"github.com/awslabs/goformation/v7/cloudformation/rds"
	"github.com/rs/zerolog/log"

	"github.com/infracost/infracost/internal/resources/aws"
	"github.com/infracost/infracost/internal/schema"
)

func GetDBInstanceRegistryItem() *schema.RegistryItem {
	return &schema.RegistryItem{
		Name: "AWS::RDS::DBInstance",
		Notes: []string{
			"DB instances are not yet supported.",
		},
		RFunc: NewDBInstance,
	}
}

func NewDBInstance(d *schema.ResourceData, u *schema.UsageData) *schema.Resource {
	cfr, ok := d.CFResource.(*rds.DBInstance)
	if !ok {
		log.Warn().Msgf("Skipping resource %s as it did not have the expected type (got %T)", d.Address, d.CFResource)
		return nil
	}

	region := "us-east-1" // TODO figure out how to set region

	licenseModel := ""
	if cfr.LicenseModel != nil {
		licenseModel = *cfr.LicenseModel
	}

	storageType := ""
	if cfr.StorageType != nil {
		storageType = *cfr.StorageType
	}

	backupRetentionPeriod := int64(1)
	if cfr.BackupRetentionPeriod != nil {
		backupRetentionPeriod = int64(*cfr.BackupRetentionPeriod)
	}

	performandeInsightsEnabled := false
	if cfr.EnablePerformanceInsights != nil {
		performandeInsightsEnabled = *cfr.EnablePerformanceInsights
	}

	isMultiAZ := false
	if cfr.MultiAZ != nil {
		isMultiAZ = *cfr.MultiAZ
	}

	dbInstanceClass := ""
	if cfr.DBInstanceClass != nil {
		dbInstanceClass = *cfr.DBInstanceClass
	}

	engine := ""
	if cfr.Engine != nil {
		engine = *cfr.Engine
	}

	iops := 0.0
	if cfr.Iops != nil {
		iops = float64(*cfr.Iops)
	}

	allocatedStorageFloat := 100.0 // TODO: What should we set this to?

	// allocatedStorage, _ := ParseFloat(*cfr.AllocatedStorage, 64)
	a := &aws.DBInstance{
		Address:                              d.Address,
		Region:                               region,
		LicenseModel:                         licenseModel,
		StorageType:                          storageType,
		BackupRetentionPeriod:                backupRetentionPeriod,
		IOOptimized:                          false, // TODO figure out how to set this
		PerformanceInsightsEnabled:           performandeInsightsEnabled,
		PerformanceInsightsLongTermRetention: false, // TODO figure out how to set this
		MultiAZ:                              isMultiAZ,
		InstanceClass:                        dbInstanceClass,
		Engine:                               engine,
		IOPS:                                 float64(iops),
		AllocatedStorageGB:                   &allocatedStorageFloat, // TODO figure out how to parse this
	}
	a.PopulateUsage(u)

	resource := a.BuildResource()

	return resource
}
