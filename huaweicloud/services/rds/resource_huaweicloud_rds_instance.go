package rds

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"

	"github.com/chnsz/golangsdk"
	"github.com/chnsz/golangsdk/openstack/bss/v2/orders"
	"github.com/chnsz/golangsdk/openstack/common/tags"
	"github.com/chnsz/golangsdk/openstack/rds/v3/backups"
	"github.com/chnsz/golangsdk/openstack/rds/v3/instances"
	"github.com/chnsz/golangsdk/openstack/rds/v3/securities"
	"github.com/huaweicloud/terraform-provider-huaweicloud/huaweicloud/common"
	"github.com/huaweicloud/terraform-provider-huaweicloud/huaweicloud/config"
	"github.com/huaweicloud/terraform-provider-huaweicloud/huaweicloud/helper/hashcode"
	"github.com/huaweicloud/terraform-provider-huaweicloud/huaweicloud/utils"
)

// ResourceRdsInstance is the impl for huaweicloud_rds_instance resource
func ResourceRdsInstance() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceRdsInstanceCreate,
		ReadContext:   resourceRdsInstanceRead,
		UpdateContext: resourceRdsInstanceUpdate,
		DeleteContext: resourceRdsInstanceDelete,
		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},

		Timeouts: &schema.ResourceTimeout{
			Create:  schema.DefaultTimeout(30 * time.Minute),
			Update:  schema.DefaultTimeout(30 * time.Minute),
			Delete:  schema.DefaultTimeout(30 * time.Minute),
			Default: schema.DefaultTimeout(15 * time.Minute),
		},

		Schema: map[string]*schema.Schema{
			"region": {
				Type:     schema.TypeString,
				Optional: true,
				Computed: true,
				ForceNew: true,
			},

			"availability_zone": {
				Type:     schema.TypeList,
				Required: true,
				ForceNew: true,
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
			},

			"name": {
				Type:     schema.TypeString,
				Required: true,
			},

			"flavor": {
				Type:     schema.TypeString,
				Required: true,
			},

			"db": {
				Type:     schema.TypeList,
				Required: true,
				ForceNew: true,
				MaxItems: 1,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"password": {
							Type:      schema.TypeString,
							Sensitive: true,
							Required:  true,
						},
						"type": {
							Type:             schema.TypeString,
							Required:         true,
							ForceNew:         true,
							DiffSuppressFunc: utils.SuppressCaseDiffs,
						},
						"version": {
							Type:     schema.TypeString,
							Required: true,
							ForceNew: true,
						},
						"port": {
							Type:     schema.TypeInt,
							Optional: true,
							Computed: true,
						},
						"user_name": {
							Type:     schema.TypeString,
							Computed: true,
						},
					},
				},
			},

			"volume": {
				Type:     schema.TypeList,
				Required: true,
				MaxItems: 1,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"size": {
							Type:     schema.TypeInt,
							Required: true,
						},
						"type": {
							Type:     schema.TypeString,
							Required: true,
							ForceNew: true,
						},
						"disk_encryption_id": {
							Type:     schema.TypeString,
							Optional: true,
							Computed: true,
							ForceNew: true,
						},
						"limit_size": {
							Type:         schema.TypeInt,
							Optional:     true,
							RequiredWith: []string{"volume.0.trigger_threshold"},
						},
						"trigger_threshold": {
							Type:         schema.TypeInt,
							Optional:     true,
							RequiredWith: []string{"volume.0.limit_size"},
						},
					},
				},
			},

			"vpc_id": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},
			"subnet_id": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},
			"security_group_id": {
				Type:     schema.TypeString,
				Required: true,
			},

			"backup_strategy": {
				Type:     schema.TypeList,
				Optional: true,
				Computed: true,
				MaxItems: 1,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"start_time": {
							Type:     schema.TypeString,
							Required: true,
						},
						"keep_days": {
							Type:     schema.TypeInt,
							Optional: true,
							Computed: true,
						},
					},
				},
			},

			"enterprise_project_id": {
				Type:     schema.TypeString,
				Optional: true,
				Computed: true,
				ForceNew: true,
			},

			"fixed_ip": {
				Type:         schema.TypeString,
				Optional:     true,
				Computed:     true,
				ForceNew:     true,
				ValidateFunc: utils.ValidateIP,
			},

			"ha_replication_mode": {
				Type:     schema.TypeString,
				Optional: true,
				Computed: true,
				ForceNew: true,
			},

			"param_group_id": {
				Type:     schema.TypeString,
				Optional: true,
				ForceNew: true,
			},

			"collation": {
				Type:     schema.TypeString,
				Optional: true,
				ForceNew: true,
			},

			"ssl_enable": {
				Type:     schema.TypeBool,
				Optional: true,
			},

			"tags": common.TagsSchema(),

			"time_zone": {
				Type:     schema.TypeString,
				Optional: true,
				Computed: true,
				ForceNew: true,
			},

			"parameters": {
				Type: schema.TypeSet,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"name": {
							Type:     schema.TypeString,
							Required: true,
						},
						"value": {
							Type:     schema.TypeString,
							Required: true,
						},
					},
				},
				Set:      parameterToHash,
				Optional: true,
				Computed: true,
			},

			"nodes": {
				Type:     schema.TypeList,
				Computed: true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"availability_zone": {
							Type:     schema.TypeString,
							Computed: true,
						},
						"id": {
							Type:     schema.TypeString,
							Computed: true,
						},
						"name": {
							Type:     schema.TypeString,
							Computed: true,
						},
						"role": {
							Type:     schema.TypeString,
							Computed: true,
						},
						"status": {
							Type:     schema.TypeString,
							Computed: true,
						},
					},
				},
			},

			"private_ips": {
				Type:     schema.TypeList,
				Computed: true,
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
			},

			"public_ips": {
				Type:     schema.TypeList,
				Computed: true,
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
			},

			"status": {
				Type:     schema.TypeString,
				Computed: true,
			},

			"created": {
				Type:     schema.TypeString,
				Computed: true,
			},

			"lower_case_table_names": {
				Type:     schema.TypeString,
				Optional: true,
				ForceNew: true,
			},

			// charge info: charging_mode, period_unit, period, auto_renew, auto_pay
			"charging_mode": common.SchemaChargingMode(nil),
			"period_unit":   common.SchemaPeriodUnit(nil),
			"period":        common.SchemaPeriod(nil),
			"auto_renew":    common.SchemaAutoRenewUpdatable(nil),
			"auto_pay":      common.SchemaAutoPay(nil),
		},
	}
}

func buildRdsInstanceDBPort(d *schema.ResourceData) string {
	if v, ok := d.GetOk("db.0.port"); ok {
		return strconv.Itoa(v.(int))
	}
	return ""
}

func isMySQLDatabase(d *schema.ResourceData) bool {
	dbType := d.Get("db.0.type").(string)
	// Database type is not case sensitive.
	return strings.ToLower(dbType) == "mysql"
}

func resourceRdsInstanceCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	config := meta.(*config.Config)
	region := config.GetRegion(d)
	client, err := config.RdsV3Client(region)
	if err != nil {
		return diag.Errorf("error creating RDS client: %s", err)
	}

	createOpts := instances.CreateOpts{
		Name:                d.Get("name").(string),
		FlavorRef:           d.Get("flavor").(string),
		VpcId:               d.Get("vpc_id").(string),
		SubnetId:            d.Get("subnet_id").(string),
		SecurityGroupId:     d.Get("security_group_id").(string),
		ConfigurationId:     d.Get("param_group_id").(string),
		TimeZone:            d.Get("time_zone").(string),
		FixedIp:             d.Get("fixed_ip").(string),
		DiskEncryptionId:    d.Get("volume.0.disk_encryption_id").(string),
		Collation:           d.Get("collation").(string),
		Port:                buildRdsInstanceDBPort(d),
		EnterpriseProjectId: config.GetEnterpriseProjectID(d),
		Region:              region,
		AvailabilityZone:    buildRdsInstanceAvailabilityZone(d),
		Datastore:           buildRdsInstanceDatastore(d),
		Volume:              buildRdsInstanceVolume(d),
		BackupStrategy:      buildRdsInstanceBackupStrategy(d),
		Ha:                  buildRdsInstanceHaReplicationMode(d),
		UnchangeableParam:   buildRdsInstanceUnchangeableParam(d),
	}

	// PrePaid
	if d.Get("charging_mode") == "prePaid" {
		if err := common.ValidatePrePaidChargeInfo(d); err != nil {
			return diag.FromErr(err)
		}

		chargeInfo := &instances.ChargeInfo{
			ChargeMode: d.Get("charging_mode").(string),
			PeriodType: d.Get("period_unit").(string),
			PeriodNum:  d.Get("period").(int),
		}
		if d.Get("auto_pay").(string) != "false" {
			chargeInfo.IsAutoPay = true
		}
		if d.Get("auto_renew").(string) == "true" {
			chargeInfo.IsAutoRenew = true
		}
		createOpts.ChargeInfo = chargeInfo
	}

	log.Printf("[DEBUG] Create Options: %#v", createOpts)
	// Add password here so it wouldn't go in the above log entry
	createOpts.Password = d.Get("db.0.password").(string)

	res, err := instances.Create(client, createOpts).Extract()
	if err != nil {
		return diag.Errorf("error creating RDS instance: %s", err)
	}
	d.SetId(res.Instance.Id)
	instanceID := d.Id()

	// wait for order success
	if res.OrderId != "" {
		bssClient, err := config.BssV2Client(config.GetRegion(d))
		if err != nil {
			return diag.Errorf("error creating BSS V2 client: %s", err)
		}
		if err := orders.WaitForOrderSuccess(bssClient, int(d.Timeout(schema.TimeoutCreate)/time.Second), res.OrderId); err != nil {
			return diag.Errorf("error waiting for RDS order %s succuss: %s", res.OrderId, err)
		}
	}

	if res.JobId != "" {
		if err := checkRDSInstanceJobFinish(client, res.JobId, d.Timeout(schema.TimeoutCreate)); err != nil {
			return diag.Errorf("error creating instance (%s): %s", instanceID, err)
		}
	}
	// for prePaid charge mode
	stateConf := &resource.StateChangeConf{
		Pending:      []string{"BUILD"},
		Target:       []string{"ACTIVE", "BACKING UP"},
		Refresh:      rdsInstanceStateRefreshFunc(client, instanceID),
		Timeout:      d.Timeout(schema.TimeoutCreate),
		Delay:        20 * time.Second,
		PollInterval: 10 * time.Second,
		// Ensure that the instance is 'ACTIVE', not going to enter 'BACKING UP'.
		ContinuousTargetOccurence: 2,
	}
	if _, err = stateConf.WaitForStateContext(ctx); err != nil {
		return diag.Errorf("error waiting for RDS instance (%s) creation completed: %s", instanceID, err)
	}

	if d.Get("ssl_enable").(bool) {
		if isMySQLDatabase(d) {
			err = configRdsInstanceSSL(d, client, d.Id())
			if err != nil {
				return diag.FromErr(err)
			}
		} else {
			return diag.Errorf("only MySQL database support SSL enable and disable")
		}
	}

	tagRaw := d.Get("tags").(map[string]interface{})
	if len(tagRaw) > 0 {
		taglist := utils.ExpandResourceTags(tagRaw)
		if tagErr := tags.Create(client, "instances", instanceID, taglist).ExtractErr(); tagErr != nil {
			return diag.Errorf("error setting tags of RDS instance (%s): %s", instanceID, tagErr)
		}
	}

	// Set Parameters
	parametersRaw := d.Get("parameters").(*schema.Set)
	if parametersRaw.Len() > 0 {
		configOpts := buildRdsInstanceParameters(parametersRaw)
		err = resource.RetryContext(ctx, d.Timeout(schema.TimeoutCreate), func() *resource.RetryError {
			_, err = instances.ModifyConfiguration(client, instanceID, configOpts).Extract()
			retryable, err := handleMultiOperationsError(err)
			if retryable {
				return resource.RetryableError(err)
			}
			if err != nil {
				return resource.NonRetryableError(err)
			}
			return nil
		})
		if err != nil {
			return diag.Errorf("error modifying parameters for RDS instance (%s): %s", instanceID, err)
		}

		// Check if we need to restart
		configs, err := instances.GetConfigurations(client, instanceID).Extract()
		if err != nil {
			return diag.Errorf("error fetching the instance parameters (%s): %s", instanceID, err)
		}

		restart := false
		for _, parameter := range parametersRaw.List() {
			name := parameter.(map[string]interface{})["name"]
			for _, v := range configs.Parameters {
				if v.Name == name {
					if v.Restart {
						restart = true
					}
					break
				}
			}
			if restart {
				break
			}
		}

		if restart {
			// If parameters which requires restart changed, reboot the instance.
			err = resource.RetryContext(ctx, d.Timeout(schema.TimeoutCreate), func() *resource.RetryError {
				_, err = instances.RebootInstance(client, instanceID).Extract()
				retryable, err := handleMultiOperationsError(err)
				if retryable {
					return resource.RetryableError(err)
				}
				if err != nil {
					return resource.NonRetryableError(err)
				}
				return nil
			})
			if err != nil {
				return diag.Errorf("error rebooting for RDS instance (%s): %s", instanceID, err)
			}

			// wait for the instance state to be 'ACTIVE'.
			stateConf := &resource.StateChangeConf{
				Target:       []string{"ACTIVE"},
				Refresh:      rdsInstanceStateRefreshFunc(client, instanceID),
				Timeout:      d.Timeout(schema.TimeoutDefault),
				Delay:        5 * time.Second,
				PollInterval: 5 * time.Second,
			}
			if _, err = stateConf.WaitForStateContext(ctx); err != nil {
				return diag.Errorf("error waiting for RDS instance (%s) become active state: %s", instanceID, err)
			}
		}
	}

	if size := d.Get("volume.0.limit_size").(int); size > 0 {
		opts := instances.EnableAutoExpandOpts{
			InstanceId:       instanceID,
			LimitSize:        size,
			TriggerThreshold: d.Get("volume.0.trigger_threshold").(int),
		}

		err = instances.EnableAutoExpand(client, opts)
		if err != nil {
			return diag.Errorf("error configuring auto-expansion: %v", err)
		}
	}

	return resourceRdsInstanceRead(ctx, d, meta)
}

func resourceRdsInstanceRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	config := meta.(*config.Config)
	client, err := config.RdsV3Client(config.GetRegion(d))
	if err != nil {
		return diag.Errorf("error creating RDS client: %s", err)
	}

	instanceID := d.Id()
	instance, err := GetRdsInstanceByID(client, instanceID)
	if err != nil {
		return diag.Errorf("error getting RDS instance: %s", err)
	}
	if instance.Id == "" {
		d.SetId("")
		return nil
	}

	log.Printf("[DEBUG] Retrieved RDS instance (%s): %#v", instanceID, instance)
	d.Set("region", instance.Region)
	d.Set("name", instance.Name)
	d.Set("status", instance.Status)
	d.Set("created", instance.Created)
	d.Set("ha_replication_mode", instance.Ha.ReplicationMode)
	d.Set("vpc_id", instance.VpcId)
	d.Set("subnet_id", instance.SubnetId)
	d.Set("security_group_id", instance.SecurityGroupId)
	d.Set("flavor", instance.FlavorRef)
	d.Set("time_zone", instance.TimeZone)
	d.Set("enterprise_project_id", instance.EnterpriseProjectId)
	d.Set("charging_mode", instance.ChargeInfo.ChargeMode)
	d.Set("tags", utils.TagsToMap(instance.Tags))

	publicIps := make([]interface{}, len(instance.PublicIps))
	for i, v := range instance.PublicIps {
		publicIps[i] = v
	}
	d.Set("public_ips", publicIps)

	privateIps := make([]string, len(instance.PrivateIps))
	for i, v := range instance.PrivateIps {
		privateIps[i] = v
	}
	d.Set("private_ips", privateIps)
	// If the creation of the RDS instance is failed, the length of the private IP list will be zero.
	if len(privateIps) > 0 {
		d.Set("fixed_ip", privateIps[0])
	}

	volume := map[string]interface{}{
		"type":               instance.Volume.Type,
		"size":               instance.Volume.Size,
		"disk_encryption_id": instance.DiskEncryptionId,
	}
	// Only MySQL engines are supported.
	resp, err := instances.GetAutoExpand(client, instanceID)
	if err != nil {
		log.Printf("[ERROR] error query automatic expansion configuration of the instance storage: %s", err)
	}
	if resp.SwitchOption {
		volume["limit_size"] = resp.LimitSize
		volume["trigger_threshold"] = resp.TriggerThreshold
	}
	if err := d.Set("volume", []map[string]interface{}{volume}); err != nil {
		return diag.Errorf("error saving volume to RDS instance (%s): %s", instanceID, err)
	}

	dbList := make([]map[string]interface{}, 1)
	database := map[string]interface{}{
		"type":      instance.DataStore.Type,
		"version":   instance.DataStore.Version,
		"port":      instance.Port,
		"user_name": instance.DbUserName,
	}
	if len(d.Get("db").([]interface{})) > 0 {
		database["password"] = d.Get("db.0.password")
	}
	dbList[0] = database
	if err := d.Set("db", dbList); err != nil {
		return diag.Errorf("error saving data base to RDS instance (%s): %s", instanceID, err)
	}

	backup := make([]map[string]interface{}, 1)
	backup[0] = map[string]interface{}{
		"start_time": instance.BackupStrategy.StartTime,
		"keep_days":  instance.BackupStrategy.KeepDays,
	}
	if err := d.Set("backup_strategy", backup); err != nil {
		return diag.Errorf("error saving backup strategy to RDS instance (%s): %s", instanceID, err)
	}

	nodes := make([]map[string]interface{}, len(instance.Nodes))
	for i, v := range instance.Nodes {
		nodes[i] = map[string]interface{}{
			"id":                v.Id,
			"name":              v.Name,
			"role":              v.Role,
			"status":            v.Status,
			"availability_zone": v.AvailabilityZone,
		}
	}
	if err := d.Set("nodes", nodes); err != nil {
		return diag.Errorf("error saving nodes to RDS instance (%s): %s", instanceID, err)
	}

	// Set Parameters
	configs, err := instances.GetConfigurations(client, instanceID).Extract()
	if err != nil {
		log.Printf("[WARN] error fetching parameters of instance (%s): %s", instanceID, err)
	} else {
		var restart []string
		var params []map[string]interface{}
		for _, parameter := range d.Get("parameters").(*schema.Set).List() {
			name := parameter.(map[string]interface{})["name"]
			for _, v := range configs.Parameters {
				if v.Name == name {
					p := map[string]interface{}{
						"name":  v.Name,
						"value": v.Value,
					}
					params = append(params, p)
					if v.Restart {
						restart = append(restart, v.Name)
					}
					break
				}
			}
		}

		if len(params) > 0 {
			if err := d.Set("parameters", params); err != nil {
				log.Printf("error saving parameters to RDS instance (%s): %s", instanceID, err)
			}
			if len(restart) > 0 && ctx.Value("parametersChanged") == "true" {
				return diag.Diagnostics{
					diag.Diagnostic{
						Severity: diag.Warning,
						Summary:  "Parameters Changed",
						Detail:   fmt.Sprintf("Parameters %s changed which needs reboot.", restart),
					},
				}
			}
		}
	}

	return nil
}

func resourceRdsInstanceUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	config := meta.(*config.Config)
	client, err := config.RdsV3Client(config.GetRegion(d))
	if err != nil {
		return diag.Errorf("error creating RDS Client: %s", err)
	}

	instanceID := d.Id()
	// Since the instance will throw an exception when making an API interface call in 'BACKING UP' state,
	// wait for the instance state to be updated to 'ACTIVE' before calling the interface.
	stateConf := &resource.StateChangeConf{
		Target:       []string{"ACTIVE"},
		Refresh:      rdsInstanceStateRefreshFunc(client, instanceID),
		Timeout:      d.Timeout(schema.TimeoutDefault),
		Delay:        5 * time.Second,
		PollInterval: 5 * time.Second,
		// Provide 10 seconds to check whether the instance is 'ACTIVE' or is about to enter 'BACKING UP'.
		ContinuousTargetOccurence: 3,
	}
	if _, err = stateConf.WaitForState(); err != nil {
		return diag.Errorf("error waiting for RDS instance (%s) become active state: %s", instanceID, err)
	}

	if err := updateRdsInstanceName(d, client, instanceID); err != nil {
		return diag.FromErr(err)
	}

	if err := updateRdsInstanceFlavor(d, config, client, instanceID, true); err != nil {
		return diag.FromErr(err)
	}

	if err := updateRdsInstanceVolumeSize(d, client, instanceID); err != nil {
		return diag.FromErr(err)
	}

	if err := updateRdsInstanceBackpStrategy(d, client, instanceID); err != nil {
		return diag.FromErr(err)
	}

	if err := updateRdsInstanceDBPort(d, client, instanceID); err != nil {
		return diag.FromErr(err)
	}

	if err := updateRdsInstanceSecurityGroup(d, client, instanceID); err != nil {
		return diag.FromErr(err)
	}

	if err := updateRdsInstanceSSLConfig(d, client, instanceID); err != nil {
		return diag.FromErr(err)
	}

	if err := updateRdsRootPassword(d, client, instanceID); err != nil {
		return diag.FromErr(err)
	}

	if d.HasChange("tags") {
		tagErr := utils.UpdateResourceTags(client, d, "instances", instanceID)
		if tagErr != nil {
			return diag.Errorf("error updating tags of RDS instance (%s): %s", instanceID, tagErr)
		}
	}

	if d.HasChange("auto_renew") {
		bssClient, err := config.BssV2Client(config.GetRegion(d))
		if err != nil {
			return diag.Errorf("error creating BSS V2 client: %s", err)
		}
		if err = common.UpdateAutoRenew(bssClient, d.Get("auto_renew").(string), d.Id()); err != nil {
			return diag.Errorf("error updating the auto-renew of the instance (%s): %s", d.Id(), err)
		}
	}

	if d.HasChange("parameters") {
		type ctxType string
		err = resource.RetryContext(ctx, d.Timeout(schema.TimeoutUpdate), func() *resource.RetryError {
			err := updateRdsParameters(d, client, instanceID)
			retryable, err := handleMultiOperationsError(err)
			if retryable {
				return resource.RetryableError(err)
			}
			if err != nil {
				return resource.NonRetryableError(err)
			}
			return nil
		})
		if err != nil {
			return diag.Errorf("error updating parameters of RDS instance (%s): %s", instanceID, err)
		}
		// Sending parametersChanged to Read to warn users the instance needs a reboot.
		ctx = context.WithValue(ctx, ctxType("parametersChanged"), "true")
	}

	if d.HasChanges("volume.0.limit_size", "volume.0.trigger_threshold") {
		limitSize := d.Get("volume.0.limit_size").(int)
		if limitSize > 0 {
			opts := instances.EnableAutoExpandOpts{
				InstanceId:       instanceID,
				LimitSize:        limitSize,
				TriggerThreshold: d.Get("volume.0.trigger_threshold").(int),
			}
			err = instances.EnableAutoExpand(client, opts)
			if err != nil {
				return diag.Errorf("an error occurred while enable automatic expansion of instance storage: %v", err)
			}
		} else {
			err = instances.DisableAutoExpand(client, instanceID)
			if err != nil {
				return diag.Errorf("an error occurred while disable automatic expansion of instance storage: %v", err)
			}
		}
	}

	return resourceRdsInstanceRead(ctx, d, meta)
}

func resourceRdsInstanceDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	config := meta.(*config.Config)
	client, err := config.RdsV3Client(config.GetRegion(d))
	if err != nil {
		return diag.Errorf("error creating rds client: %s ", err)
	}

	id := d.Id()
	log.Printf("[DEBUG] Deleting Instance %s", id)
	if v, ok := d.GetOk("charging_mode"); ok && v.(string) == "prePaid" {
		if err := common.UnsubscribePrePaidResource(d, config, []string{id}); err != nil {
			return diag.Errorf("error unsubscribe RDS instance: %s", err)
		}
	} else {
		result := instances.Delete(client, id)
		if result.Err != nil {
			return diag.FromErr(result.Err)
		}
	}

	stateConf := &resource.StateChangeConf{
		Pending:      []string{"ACTIVE"},
		Target:       []string{"DELETED"},
		Refresh:      rdsInstanceStateRefreshFunc(client, id),
		Timeout:      d.Timeout(schema.TimeoutDelete),
		Delay:        15 * time.Second,
		PollInterval: 5 * time.Second,
	}

	_, err = stateConf.WaitForStateContext(ctx)
	if err != nil {
		return diag.Errorf(
			"error waiting for rds instance (%s) to be deleted: %s ",
			id, err)
	}

	log.Printf("[DEBUG] Successfully deleted RDS instance %s", id)
	return nil
}

func GetRdsInstanceByID(client *golangsdk.ServiceClient, instanceID string) (*instances.RdsInstanceResponse, error) {
	listOpts := instances.ListOpts{
		Id: instanceID,
	}
	pages, err := instances.List(client, listOpts).AllPages()
	if err != nil {
		return nil, fmt.Errorf("An error occurred while querying rds instance %s: %s", instanceID, err)
	}

	resp, err := instances.ExtractRdsInstances(pages)
	if err != nil {
		return nil, err
	}

	instanceList := resp.Instances
	if len(instanceList) == 0 {
		// return an empty rds instance
		log.Printf("[WARN] can not find the specified rds instance %s", instanceID)
		instance := new(instances.RdsInstanceResponse)
		return instance, nil
	}

	if len(instanceList) > 1 {
		return nil, fmt.Errorf("retrieving more than one rds instance by %s", instanceID)
	}
	if instanceList[0].Id != instanceID {
		return nil, fmt.Errorf("the id of rds instance was expected %s, but got %s",
			instanceID, instanceList[0].Id)
	}

	return &instanceList[0], nil
}

func buildRdsInstanceAvailabilityZone(d *schema.ResourceData) string {
	azList := make([]string, len(d.Get("availability_zone").([]interface{})))
	for i, az := range d.Get("availability_zone").([]interface{}) {
		azList[i] = az.(string)
	}
	return strings.Join(azList, ",")
}

func buildRdsInstanceDatastore(d *schema.ResourceData) *instances.Datastore {
	var database *instances.Datastore
	dbRaw := d.Get("db").([]interface{})

	if len(dbRaw) == 1 {
		database = new(instances.Datastore)
		database.Type = dbRaw[0].(map[string]interface{})["type"].(string)
		database.Version = dbRaw[0].(map[string]interface{})["version"].(string)
	}
	return database
}

func buildRdsInstanceVolume(d *schema.ResourceData) *instances.Volume {
	var volume *instances.Volume
	volumeRaw := d.Get("volume").([]interface{})

	if len(volumeRaw) == 1 {
		volume = new(instances.Volume)
		volume.Type = volumeRaw[0].(map[string]interface{})["type"].(string)
		volume.Size = volumeRaw[0].(map[string]interface{})["size"].(int)
	}
	return volume
}

func buildRdsInstanceBackupStrategy(d *schema.ResourceData) *instances.BackupStrategy {
	var backupStrategy *instances.BackupStrategy
	backupRaw := d.Get("backup_strategy").([]interface{})

	if len(backupRaw) == 1 {
		backupStrategy = new(instances.BackupStrategy)
		backupStrategy.StartTime = backupRaw[0].(map[string]interface{})["start_time"].(string)
		backupStrategy.KeepDays = backupRaw[0].(map[string]interface{})["keep_days"].(int)
	}
	return backupStrategy
}

func buildRdsInstanceUnchangeableParam(d *schema.ResourceData) *instances.UnchangeableParam {
	var unchangeableParam *instances.UnchangeableParam
	if v, ok := d.GetOk("lower_case_table_names"); ok {
		unchangeableParam = new(instances.UnchangeableParam)
		unchangeableParam.LowerCaseTableNames = v.(string)
	}
	return unchangeableParam
}

func buildRdsInstanceHaReplicationMode(d *schema.ResourceData) *instances.Ha {
	var ha *instances.Ha
	if v, ok := d.GetOk("ha_replication_mode"); ok {
		ha = new(instances.Ha)
		ha.Mode = "ha"
		ha.ReplicationMode = v.(string)
	}
	return ha
}

func buildRdsInstanceParameters(params *schema.Set) instances.ModifyConfigurationOpts {
	var configOpts instances.ModifyConfigurationOpts

	values := make(map[string]string)
	for _, v := range params.List() {
		key := v.(map[string]interface{})["name"].(string)
		value := v.(map[string]interface{})["value"].(string)
		values[key] = value
	}
	configOpts.Values = values
	return configOpts
}

func updateRdsInstanceName(d *schema.ResourceData, client *golangsdk.ServiceClient, instanceID string) error {
	if !d.HasChange("name") {
		return nil
	}

	renameOpts := instances.RenameInstanceOpts{
		Name: d.Get("name").(string),
	}
	r := instances.Rename(client, renameOpts, instanceID)
	if r.Result.Err != nil {
		return fmt.Errorf("error renaming RDS instance (%s): %s", instanceID, r.Err)
	}

	stateConf := &resource.StateChangeConf{
		Pending:      []string{"MODIFYING"},
		Target:       []string{"ACTIVE"},
		Refresh:      rdsInstanceStateRefreshFunc(client, instanceID),
		Timeout:      d.Timeout(schema.TimeoutUpdate),
		Delay:        5 * time.Second,
		PollInterval: 3 * time.Second,
	}
	if _, err := stateConf.WaitForState(); err != nil {
		return fmt.Errorf("error waiting for RDS instance (%s) flavor to be updated: %s ", instanceID, err)
	}

	return nil
}

func updateRdsInstanceFlavor(d *schema.ResourceData, config *config.Config, client *golangsdk.ServiceClient,
	instanceID string, isSupportAutoPay bool) error {
	if !d.HasChange("flavor") {
		return nil
	}

	resizeFlavor := instances.SpecCode{
		Speccode:  d.Get("flavor").(string),
		IsAutoPay: true,
	}
	if isSupportAutoPay && d.Get("auto_pay").(string) == "false" {
		resizeFlavor.IsAutoPay = false
	}
	var resizeFlavorOpts instances.ResizeFlavorOpts
	resizeFlavorOpts.ResizeFlavor = &resizeFlavor

	res, err := instances.Resize(client, resizeFlavorOpts, instanceID).Extract()
	if err != nil {
		return fmt.Errorf("error updating instance Flavor from result: %s ", err)
	}

	// wait for order success
	if res.OrderId != "" {
		bssClient, err := config.BssV2Client(config.GetRegion(d))
		if err != nil {
			return fmt.Errorf("error creating BSS V2 client: %s", err)
		}
		if err := orders.WaitForOrderSuccess(bssClient, int(d.Timeout(schema.TimeoutUpdate)/time.Second), res.OrderId); err != nil {
			return fmt.Errorf("error waiting for RDS order %s succuss: %s", res.OrderId, err)
		}
	}

	stateConf := &resource.StateChangeConf{
		Pending:      []string{"MODIFYING"},
		Target:       []string{"ACTIVE"},
		Refresh:      rdsInstanceStateRefreshFunc(client, instanceID),
		Timeout:      d.Timeout(schema.TimeoutUpdate),
		Delay:        15 * time.Second,
		PollInterval: 15 * time.Second,
	}
	if _, err = stateConf.WaitForState(); err != nil {
		return fmt.Errorf("error waiting for instance (%s) flavor to be Updated: %s ", instanceID, err)
	}
	return nil
}

func updateRdsInstanceVolumeSize(d *schema.ResourceData, client *golangsdk.ServiceClient, instanceID string) error {
	if !d.HasChange("volume.0.size") {
		return nil
	}

	volumeRaw := d.Get("volume").([]interface{})
	volumeItem := volumeRaw[0].(map[string]interface{})
	enlargeOpts := instances.EnlargeVolumeOpts{
		EnlargeVolume: &instances.EnlargeVolumeSize{
			Size: volumeItem["size"].(int),
		},
	}

	log.Printf("[DEBUG] Enlarge Volume opts: %+v", enlargeOpts)
	instance, err := instances.EnlargeVolume(client, enlargeOpts, instanceID).Extract()
	if err != nil {
		return fmt.Errorf("error updating instance volume from result: %s ", err)
	}
	if err := checkRDSInstanceJobFinish(client, instance.JobId, d.Timeout(schema.TimeoutUpdate)); err != nil {
		return fmt.Errorf("error updating instance (%s): %s", instanceID, err)
	}

	return nil
}

func updateRdsInstanceBackpStrategy(d *schema.ResourceData, client *golangsdk.ServiceClient, instanceID string) error {
	if !d.HasChange("backup_strategy") {
		return nil
	}

	backupRaw := d.Get("backup_strategy").([]interface{})
	rawMap := backupRaw[0].(map[string]interface{})
	keepDays := rawMap["keep_days"].(int)

	updateOpts := backups.UpdateOpts{
		KeepDays:  &keepDays,
		StartTime: rawMap["start_time"].(string),
		Period:    "1,2,3,4,5,6,7",
	}

	log.Printf("[DEBUG] updateOpts: %#v", updateOpts)
	err := backups.Update(client, instanceID, updateOpts).ExtractErr()
	if err != nil {
		return fmt.Errorf("error updating RDS instance backup strategy (%s): %s", instanceID, err)
	}

	stateConf := &resource.StateChangeConf{
		Pending:      []string{"BACKING UP"},
		Target:       []string{"ACTIVE"},
		Refresh:      rdsInstanceStateRefreshFunc(client, instanceID),
		Timeout:      d.Timeout(schema.TimeoutUpdate),
		Delay:        15 * time.Second,
		PollInterval: 3 * time.Second,
	}
	if _, err = stateConf.WaitForState(); err != nil {
		return fmt.Errorf("error waiting for RDS instance (%s) backup strategy to be updated: %s ", instanceID, err)
	}

	return nil
}

func updateRdsInstanceDBPort(d *schema.ResourceData, client *golangsdk.ServiceClient, instanceID string) error {
	if !d.HasChange("db.0.port") {
		return nil
	}

	udpateOpts := securities.PortOpts{
		Port: d.Get("db.0.port").(int),
	}
	log.Printf("[DEBUG] Update opts of Database port: %+v", udpateOpts)
	_, err := securities.UpdatePort(client, instanceID, udpateOpts).Extract()
	if err != nil {
		return fmt.Errorf("error updating instance database port: %s ", err)
	}
	// for prePaid charge mode
	stateConf := &resource.StateChangeConf{
		Pending:      []string{"MODIFYING DATABASE PORT"},
		Target:       []string{"ACTIVE"},
		Refresh:      rdsInstanceStateRefreshFunc(client, instanceID),
		Timeout:      d.Timeout(schema.TimeoutUpdate),
		Delay:        5 * time.Second,
		PollInterval: 3 * time.Second,
	}
	if _, err = stateConf.WaitForState(); err != nil {
		return fmt.Errorf("error waiting for RDS instance (%s) creation completed: %s", instanceID, err)
	}

	return nil
}

func updateRdsInstanceSecurityGroup(d *schema.ResourceData, client *golangsdk.ServiceClient, instanceID string) error {
	if !d.HasChange("security_group_id") {
		return nil
	}

	udpateOpts := securities.SecGroupOpts{
		SecurityGroupId: d.Get("security_group_id").(string),
	}
	log.Printf("[DEBUG] Update opts of security group: %+v", udpateOpts)
	_, err := securities.UpdateSecGroup(client, instanceID, udpateOpts).Extract()
	if err != nil {
		return fmt.Errorf("error updating instance security group: %s ", err)
	}

	return nil
}

func updateRdsInstanceSSLConfig(d *schema.ResourceData, client *golangsdk.ServiceClient, instanceID string) error {
	if !d.HasChange("ssl_enable") {
		return nil
	}
	if !isMySQLDatabase(d) {
		return fmt.Errorf("only MySQL database support SSL enable and disable")
	}
	return configRdsInstanceSSL(d, client, instanceID)
}

func updateRdsParameters(d *schema.ResourceData, client *golangsdk.ServiceClient, instanceID string) error {
	values := make(map[string]string)

	o, n := d.GetChange("parameters")
	os, ns := o.(*schema.Set), n.(*schema.Set)
	change := ns.Difference(os).List()
	if len(change) > 0 {
		for _, v := range change {
			key := v.(map[string]interface{})["name"].(string)
			value := v.(map[string]interface{})["value"].(string)
			values[key] = value
		}

		configOpts := instances.ModifyConfigurationOpts{
			Values: values,
		}
		_, err := instances.ModifyConfiguration(client, instanceID, configOpts).Extract()
		if err != nil {
			return fmt.Errorf("error modifying parameters for RDS instance (%s): %s", instanceID, err)
		}
	}

	return nil
}

func configRdsInstanceSSL(d *schema.ResourceData, client *golangsdk.ServiceClient, instanceID string) error {
	sslEnable := d.Get("ssl_enable").(bool)
	udpateOpts := securities.SSLOpts{
		SSLEnable: &sslEnable,
	}
	log.Printf("[DEBUG] Update opts of SSL configuration: %+v", udpateOpts)
	err := securities.UpdateSSL(client, instanceID, udpateOpts).ExtractErr()
	if err != nil {
		return fmt.Errorf("error updating instance SSL configuration: %s ", err)
	}
	return nil
}

func checkRDSInstanceJobFinish(client *golangsdk.ServiceClient, jobID string, timeout time.Duration) error {
	stateConf := &resource.StateChangeConf{
		Pending:      []string{"Running"},
		Target:       []string{"Completed"},
		Refresh:      rdsInstanceJobRefreshFunc(client, jobID),
		Timeout:      timeout,
		Delay:        20 * time.Second,
		PollInterval: 10 * time.Second,
	}
	if _, err := stateConf.WaitForState(); err != nil {
		return fmt.Errorf("error waiting for RDS instance (%s) job to be completed: %s ", jobID, err)
	}
	return nil
}

func rdsInstanceJobRefreshFunc(client *golangsdk.ServiceClient, jobID string) resource.StateRefreshFunc {
	return func() (interface{}, string, error) {
		jobOpts := instances.RDSJobOpts{
			JobID: jobID,
		}
		jobList, err := instances.GetRDSJob(client, jobOpts).Extract()
		if err != nil {
			return nil, "FOUND ERROR", err
		}

		return jobList.Job, jobList.Job.Status, nil
	}
}

func rdsInstanceStateRefreshFunc(client *golangsdk.ServiceClient, instanceID string) resource.StateRefreshFunc {
	return func() (interface{}, string, error) {
		instance, err := GetRdsInstanceByID(client, instanceID)
		if err != nil {
			return nil, "FOUND ERROR", err
		}
		if instance.Id == "" {
			return instance, "DELETED", nil
		}

		return instance, instance.Status, nil
	}
}

func updateRdsRootPassword(d *schema.ResourceData, client *golangsdk.ServiceClient, instanceID string) error {
	if !d.HasChange("db.0.password") {
		return nil
	}

	updateOpts := instances.RestRootPasswordOpts{
		DbUserPwd: d.Get("db.0.password").(string),
	}
	_, err := instances.RestRootPassword(client, instanceID, updateOpts)
	if err != nil {
		return fmt.Errorf("error resetting the root password: %s", err)
	}
	return nil
}

func parameterToHash(v interface{}) int {
	m := v.(map[string]interface{})
	return hashcode.String(m["name"].(string) + m["value"].(string))
}
