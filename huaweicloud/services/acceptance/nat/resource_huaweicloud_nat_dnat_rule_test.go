package nat

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"

	"github.com/chnsz/golangsdk/openstack/nat/v2/dnats"

	"github.com/huaweicloud/terraform-provider-huaweicloud/huaweicloud/config"
	"github.com/huaweicloud/terraform-provider-huaweicloud/huaweicloud/services/acceptance"
	"github.com/huaweicloud/terraform-provider-huaweicloud/huaweicloud/services/acceptance/common"
)

func getPublicDnatRuleResourceFunc(cfg *config.Config, state *terraform.ResourceState) (interface{}, error) {
	client, err := cfg.NatGatewayClient(acceptance.HW_REGION_NAME)
	if err != nil {
		return nil, fmt.Errorf("error creating NAT v2 client: %s", err)
	}

	return dnats.Get(client, state.Primary.ID)
}

func TestAccPublicDnatRule_basic(t *testing.T) {
	var (
		obj dnats.Rule

		rName = "huaweicloud_nat_dnat_rule.test"
		name  = acceptance.RandomAccResourceNameWithDash()
	)

	rc := acceptance.InitResourceCheck(
		rName,
		&obj,
		getPublicDnatRuleResourceFunc,
	)

	resource.ParallelTest(t, resource.TestCase{
		PreCheck: func() {
			acceptance.TestAccPreCheck(t)
		},
		ProviderFactories: acceptance.TestAccProviderFactories,
		CheckDestroy:      rc.CheckResourceDestroy(),
		Steps: []resource.TestStep{
			{
				Config: testAccPublicDnatRule_basic_step_1(name),
				Check: resource.ComposeTestCheckFunc(
					rc.CheckResourceExists(),
					resource.TestCheckResourceAttrPair(rName, "nat_gateway_id", "huaweicloud_nat_gateway.test", "id"),
					resource.TestCheckResourceAttrPair(rName, "floating_ip_id", "huaweicloud_vpc_eip.test", "id"),
					resource.TestCheckResourceAttrPair(rName, "private_ip", "huaweicloud_compute_instance.test", "network.0.fixed_ip_v4"),
					resource.TestCheckResourceAttr(rName, "protocol", "udp"),
					resource.TestCheckResourceAttr(rName, "description", "Created by acc test"),
					resource.TestCheckResourceAttr(rName, "internal_service_port", "80"),
					resource.TestCheckResourceAttr(rName, "external_service_port", "8080"),
				),
			},
			{
				Config: testAccPublicDnatRule_basic_step_2(name),
				Check: resource.ComposeTestCheckFunc(
					rc.CheckResourceExists(),
					resource.TestCheckResourceAttr(rName, "protocol", "any"),
					resource.TestCheckResourceAttr(rName, "internal_service_port", "0"),
					resource.TestCheckResourceAttr(rName, "external_service_port", "0"),
				),
			},
			{
				Config: testAccPublicDnatRule_basic_step_3(name),
				Check: resource.ComposeTestCheckFunc(
					rc.CheckResourceExists(),
					resource.TestCheckResourceAttr(rName, "protocol", "tcp"),
					resource.TestCheckResourceAttr(rName, "internal_service_port", "0"),
					resource.TestCheckResourceAttr(rName, "external_service_port", "0"),
					resource.TestCheckResourceAttr(rName, "internal_service_port_range", "23-823"),
					resource.TestCheckResourceAttr(rName, "external_service_port_range", "8023-8823"),
				),
			},
			{
				ResourceName:      rName,
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func testAccPublicDnatRule_base(name string) string {
	return fmt.Sprintf(`
%[1]s

resource "huaweicloud_vpc_eip" "test" {
  publicip {
    type = "5_bgp"
  }

  bandwidth {
    name        = "%[2]s"
    size        = 5
    share_type  = "PER"
    charge_mode = "traffic"
  }
}

resource "huaweicloud_compute_instance" "test" {
  name               = "%[2]s"
  image_id           = data.huaweicloud_images_image.test.id
  flavor_id          = data.huaweicloud_compute_flavors.test.ids[0]
  security_group_ids = [huaweicloud_networking_secgroup.test.id]
  availability_zone  = data.huaweicloud_availability_zones.test.names[0]

  network {
    uuid = huaweicloud_vpc_subnet.test.id
  }
}

resource "huaweicloud_nat_gateway" "test" {
  name                  = "%[2]s"
  spec                  = "2"
  vpc_id                = huaweicloud_vpc.test.id
  subnet_id             = huaweicloud_vpc_subnet.test.id
  enterprise_project_id = "0"
}
`, common.TestBaseComputeResources(name), name)
}

func testAccPublicDnatRule_basic_step_1(name string) string {
	return fmt.Sprintf(`
%[1]s

resource "huaweicloud_nat_dnat_rule" "test" {
  nat_gateway_id        = huaweicloud_nat_gateway.test.id
  floating_ip_id        = huaweicloud_vpc_eip.test.id
  private_ip            = huaweicloud_compute_instance.test.network[0].fixed_ip_v4
  description           = "Created by acc test"
  protocol              = "udp"
  internal_service_port = 80
  external_service_port = 8080
}
`, testAccPublicDnatRule_base(name))
}

func testAccPublicDnatRule_basic_step_2(name string) string {
	return fmt.Sprintf(`
%[1]s

resource "huaweicloud_nat_dnat_rule" "test" {
  nat_gateway_id        = huaweicloud_nat_gateway.test.id
  floating_ip_id        = huaweicloud_vpc_eip.test.id
  private_ip            = huaweicloud_compute_instance.test.network[0].fixed_ip_v4
  description           = ""
  protocol              = "any"
  internal_service_port = 0
  external_service_port = 0
}
`, testAccPublicDnatRule_base(name))
}

func testAccPublicDnatRule_basic_step_3(name string) string {
	return fmt.Sprintf(`
%[1]s

resource "huaweicloud_nat_dnat_rule" "test" {
  nat_gateway_id                  = huaweicloud_nat_gateway.test.id
  floating_ip_id              = huaweicloud_vpc_eip.test.id
  private_ip                  = huaweicloud_compute_instance.test.network[0].fixed_ip_v4
  protocol                    = "tcp"
  internal_service_port_range = "23-823"
  external_service_port_range = "8023-8823"
}
`, testAccPublicDnatRule_base(name))
}
