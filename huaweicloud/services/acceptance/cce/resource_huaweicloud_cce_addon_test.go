package cce

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/acctest"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"

	"github.com/chnsz/golangsdk/openstack/cce/v3/addons"

	"github.com/huaweicloud/terraform-provider-huaweicloud/huaweicloud/config"
	"github.com/huaweicloud/terraform-provider-huaweicloud/huaweicloud/services/acceptance"
)

func TestAccAddon_basic(t *testing.T) {
	var addon addons.Addon

	rName := fmt.Sprintf("tf-acc-test-%s", acctest.RandString(5))
	resourceName := "huaweicloud_cce_addon.test"
	clusterName := "huaweicloud_cce_cluster.test"

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:          func() { acceptance.TestAccPreCheck(t) },
		ProviderFactories: acceptance.TestAccProviderFactories,
		CheckDestroy:      testAccCheckAddonDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccAddon_basic(rName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckAddonExists(resourceName, clusterName, &addon),
					resource.TestCheckResourceAttrPair(resourceName, "cluster_id",
						"huaweicloud_cce_cluster.test", "id"),
					resource.TestCheckResourceAttrSet(resourceName, "version"),
					resource.TestCheckResourceAttr(resourceName, "template_name", "metrics-server"),
					resource.TestCheckResourceAttr(resourceName, "status", "running"),
				),
			},
			{
				ResourceName:      resourceName,
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateIdFunc: testAccAddonImportStateIdFunc(),
			},
		},
	})
}

func TestAccAddon_values(t *testing.T) {
	var addon addons.Addon

	rName := fmt.Sprintf("tf-acc-test-%s", acctest.RandString(5))
	resourceName := "huaweicloud_cce_addon.test"
	clusterName := "huaweicloud_cce_cluster.test"

	resource.ParallelTest(t, resource.TestCase{
		PreCheck: func() {
			acceptance.TestAccPreCheck(t)
			acceptance.TestAccPreCheckProjectID(t)
		},
		ProviderFactories: acceptance.TestAccProviderFactories,
		CheckDestroy:      testAccCheckAddonDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccAddon_values(rName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckAddonExists(resourceName, clusterName, &addon),
					resource.TestCheckResourceAttrPair(resourceName, "cluster_id",
						"huaweicloud_cce_cluster.test", "id"),
					resource.TestCheckResourceAttr(resourceName, "version", "1.25.21"),
					resource.TestCheckResourceAttr(resourceName, "template_name", "autoscaler"),
					resource.TestCheckResourceAttr(resourceName, "status", "running"),
				),
			},
			{
				Config: testAccAddon_values_update(rName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckAddonExists(resourceName, clusterName, &addon),
					// the values not set, only check if the updating request is successful
					resource.TestCheckResourceAttrPair(resourceName, "cluster_id",
						"huaweicloud_cce_cluster.test", "id"),
					resource.TestCheckResourceAttr(resourceName, "version", "1.25.21"),
					resource.TestCheckResourceAttr(resourceName, "template_name", "autoscaler"),
					resource.TestCheckResourceAttr(resourceName, "status", "running"),
				),
			},
		},
	})
}

func testAccCheckAddonDestroy(s *terraform.State) error {
	cfg := acceptance.TestAccProvider.Meta().(*config.Config)
	cceClient, err := cfg.CceAddonV3Client(acceptance.HW_REGION_NAME)
	if err != nil {
		return fmt.Errorf("error creating CCE Addon client: %s", err)
	}

	var clusterId string

	for _, rs := range s.RootModule().Resources {
		if rs.Type == "huaweicloud_cce_cluster" {
			clusterId = rs.Primary.ID
		}

		if rs.Type != "huaweicloud_cce_addon" {
			continue
		}

		if clusterId != "" {
			_, err := addons.Get(cceClient, rs.Primary.ID, clusterId).Extract()
			if err == nil {
				return fmt.Errorf("addon still exists")
			}
		}
	}
	return nil
}

func testAccCheckAddonExists(n string, cluster string, addon *addons.Addon) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[n]
		if !ok {
			return fmt.Errorf("not found: %s", n)
		}
		c, ok := s.RootModule().Resources[cluster]
		if !ok {
			return fmt.Errorf("cluster not found: %s", c)
		}

		if rs.Primary.ID == "" {
			return fmt.Errorf("no ID is set")
		}
		if c.Primary.ID == "" {
			return fmt.Errorf("cluster id is not set")
		}

		cfg := acceptance.TestAccProvider.Meta().(*config.Config)
		cceClient, err := cfg.CceAddonV3Client(acceptance.HW_REGION_NAME)
		if err != nil {
			return fmt.Errorf("error creating CCE Addon client: %s", err)
		}

		found, err := addons.Get(cceClient, rs.Primary.ID, c.Primary.ID).Extract()
		if err != nil {
			return err
		}

		if found.Metadata.Id != rs.Primary.ID {
			return fmt.Errorf("addon not found")
		}

		*addon = *found

		return nil
	}
}

func testAccAddonImportStateIdFunc() resource.ImportStateIdFunc {
	return func(s *terraform.State) (string, error) {
		var clusterID string
		var addonID string
		for _, rs := range s.RootModule().Resources {
			if rs.Type == "huaweicloud_cce_cluster" {
				clusterID = rs.Primary.ID
			} else if rs.Type == "huaweicloud_cce_addon" {
				addonID = rs.Primary.ID
			}
		}
		if clusterID == "" || addonID == "" {
			return "", fmt.Errorf("resource not found: %s/%s", clusterID, addonID)
		}
		return fmt.Sprintf("%s/%s", clusterID, addonID), nil
	}
}

func testAccAddon_Base(rName string) string {
	return fmt.Sprintf(`
%s

resource "huaweicloud_cce_node" "test" {
  cluster_id        = huaweicloud_cce_cluster.test.id
  name              = "%s"
  flavor_id         = "c7.large.4"
  availability_zone = data.huaweicloud_availability_zones.test.names[0]
  key_pair          = huaweicloud_compute_keypair.test.name

  root_volume {
    size       = 40
    volumetype = "SSD"
  }
  data_volumes {
    size       = 100
    volumetype = "SSD"
  }
}
`, testAccNode_Base(rName), rName)
}

func testAccAddon_basic(rName string) string {
	return fmt.Sprintf(`
%s

resource "huaweicloud_cce_addon" "test" {
  cluster_id    = huaweicloud_cce_cluster.test.id
  template_name = "metrics-server"
  depends_on    = [huaweicloud_cce_node.test]
}
`, testAccAddon_Base(rName))
}

func testAccAddon_values_base(rName string) string {
	return fmt.Sprintf(`
%s

resource "huaweicloud_cce_node_pool" "test" {
  cluster_id         = huaweicloud_cce_cluster.test.id
  name               = "%s"
  os                 = "EulerOS 2.5"
  flavor_id          = "c7.large.4"
  initial_node_count = 4
  availability_zone  = data.huaweicloud_availability_zones.test.names[0]
  key_pair           = huaweicloud_compute_keypair.test.name
  scall_enable       = true
  min_node_count     = 2
  max_node_count     = 10
  priority           = 1
  type               = "vm"

  root_volume {
    size       = 40
    volumetype = "SSD"
  }
  data_volumes {
    size       = 100
    volumetype = "SSD"
  }
}

data "huaweicloud_cce_addon_template" "test" {
  cluster_id = huaweicloud_cce_cluster.test.id
  name       = "autoscaler"
  version    = "1.25.21"
}
`, testAccCCENodePool_Base(rName), rName)
}

func testAccAddon_values(rName string) string {
	return fmt.Sprintf(`
%s

resource "huaweicloud_cce_addon" "test" {
  cluster_id    = huaweicloud_cce_cluster.test.id
  template_name = "autoscaler"
  version       = "1.25.21"

  values {
    basic       = jsondecode(data.huaweicloud_cce_addon_template.test.spec).basic
    custom_json = jsonencode(merge(
      jsondecode(data.huaweicloud_cce_addon_template.test.spec).parameters.custom,
      {
        cluster_id = huaweicloud_cce_cluster.test.id
        tenant_id  = "%s"
        logLevel   = 3
      }
    ))
    flavor_json = jsonencode(jsondecode(data.huaweicloud_cce_addon_template.test.spec).parameters.flavor1)
  }
  
  depends_on = [
    huaweicloud_cce_node_pool.test,
  ]
}
`, testAccAddon_values_base(rName), acceptance.HW_PROJECT_ID)
}

func testAccAddon_values_update(rName string) string {
	return fmt.Sprintf(`
%s

resource "huaweicloud_cce_addon" "test" {
  cluster_id    = huaweicloud_cce_cluster.test.id
  template_name = "autoscaler"
  version       = "1.25.21"

  values {
    basic       = jsondecode(data.huaweicloud_cce_addon_template.test.spec).basic
    custom_json = jsonencode(merge(
      jsondecode(data.huaweicloud_cce_addon_template.test.spec).parameters.custom,
      {
        cluster_id = huaweicloud_cce_cluster.test.id
        tenant_id  = "%s"
        logLevel   = 4
      }
    ))
    flavor_json = jsonencode(jsondecode(data.huaweicloud_cce_addon_template.test.spec).parameters.flavor2)
  }
  
  depends_on = [
    huaweicloud_cce_node_pool.test,
  ]
}
`, testAccAddon_values_base(rName), acceptance.HW_PROJECT_ID)
}
