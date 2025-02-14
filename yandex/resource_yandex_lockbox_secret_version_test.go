package yandex

import (
	"context"
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/acctest"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
	"github.com/yandex-cloud/go-genproto/yandex/cloud/lockbox/v1"
)

func TestAccLockboxVersion_basic(t *testing.T) {
	secretName := "a" + acctest.RandString(10)
	secretDesc := "Terraform test secret"
	versionDesc := "Terraform test version"
	secretResource := "yandex_lockbox_secret.basic_secret"
	versionResource := "yandex_lockbox_secret_version.basic_version"
	versionID := ""
	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: testAccProviderFactories,
		CheckDestroy:      testAccCheckYandexLockboxSecretAllDestroyed,
		Steps: []resource.TestStep{
			{
				// Create secret and version
				Config: testAccLockboxSecretVersionBasic(secretName, secretDesc, versionDesc),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckYandexLockboxResourceExists(secretResource, nil),
					testAccCheckYandexLockboxResourceExists(versionResource, &versionID), // stores current versionID
					resource.TestCheckResourceAttr(versionResource, "description", versionDesc),
					testAccCheckYandexLockboxVersionEntries(versionResource, []*lockboxEntryCheck{
						{Key: "key1", Val: "val1"},
						{Key: "key2", Val: "val2"},
					}),
				),
			},
			{
				// update version description (will add a new version to the secret)
				Config: testAccLockboxSecretVersionBasic(secretName, secretDesc, versionDesc+" updated"),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckYandexLockboxResourceExists(secretResource, nil),
					testAccCheckYandexLockboxResourceExists(versionResource, &versionID), // checks that now versionID is different
					resource.TestCheckResourceAttr(versionResource, "description", versionDesc+" updated"),
					testAccCheckYandexLockboxVersionEntries(versionResource, []*lockboxEntryCheck{
						{Key: "key1", Val: "val1"},
						{Key: "key2", Val: "val2"},
					}),
				),
			},
		},
	})
}

func TestAccLockboxVersion_update_entries(t *testing.T) {
	secretName := "a" + acctest.RandString(10)
	secretDesc := "Terraform test secret"
	versionDesc := "Terraform test version"
	secretResource := "yandex_lockbox_secret.basic_secret"
	versionResource := "yandex_lockbox_secret_version.basic_version"
	versionID := ""
	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: testAccProviderFactories,
		CheckDestroy:      testAccCheckYandexLockboxSecretAllDestroyed,
		Steps: []resource.TestStep{
			{
				// Create secret and version
				Config: testAccLockboxSecretVersion(secretName, secretDesc, versionDesc, []*lockboxEntryCheck{
					{Key: "key1", Val: "val1"},
					{Key: "key2", Val: "val2"},
				}),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckYandexLockboxResourceExists(secretResource, nil),
					testAccCheckYandexLockboxResourceExists(versionResource, &versionID),
					resource.TestCheckResourceAttr(versionResource, "description", versionDesc),
					testAccCheckYandexLockboxVersionEntries(versionResource, []*lockboxEntryCheck{
						{Key: "key1", Val: "val1"},
						{Key: "key2", Val: "val2"},
					}),
				),
			},
			{
				// modify entries
				Config: testAccLockboxSecretVersion(secretName, secretDesc, versionDesc, []*lockboxEntryCheck{
					// {Key: "key1", Val: "val1"}, // remove
					{Key: "key2", Val: "val22"}, // modify
					{Key: "key3", Val: "val3"},  // add
				}),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckYandexLockboxResourceExists(secretResource, &versionID),
					resource.TestCheckResourceAttr(versionResource, "description", versionDesc),
					testAccCheckYandexLockboxVersionEntries(versionResource, []*lockboxEntryCheck{
						{Key: "key2", Val: "val22"},
						{Key: "key3", Val: "val3"},
					}),
				),
			},
		},
	})
}

func TestAccLockboxVersion_command(t *testing.T) {
	secretName := "a" + acctest.RandString(10)
	versionResource := "yandex_lockbox_secret_version.exec_version"
	versionID := ""
	script := "test-fixtures/fake_secret_generator.sh"
	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: testAccProviderFactories,
		CheckDestroy:      testAccCheckYandexLockboxSecretAllDestroyed,
		Steps: []resource.TestStep{
			{
				// Create secret and version
				Config: testAccLockboxSecretVersionWithCommand(secretName, "echo", "dummy value", ""),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckYandexLockboxResourceExists(versionResource, &versionID),
					testAccCheckYandexLockboxVersionEntries(versionResource, []*lockboxEntryCheck{
						{Key: "k1", Val: "dummy value\n"},
						{Key: "k2", Val: "plain value"},
					}),
				),
			},
			{
				// Change exec values (change arg)
				Config: testAccLockboxSecretVersionWithCommand(secretName, script, "other value", "just a var"),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckYandexLockboxResourceExists(versionResource, &versionID),
					testAccCheckYandexLockboxVersionEntries(versionResource, []*lockboxEntryCheck{
						{Key: "k1", Regexp: regexp.MustCompile(`arg: other value, var: just a var, rnd: \d+`)},
						{Key: "k2", Val: "plain value"},
					}),
				),
			},
			{
				// Change exec values (remove args and env)
				Config: testAccLockboxSecretVersionWithCommand(secretName, script, "", ""),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckYandexLockboxResourceExists(versionResource, &versionID),
					testAccCheckYandexLockboxVersionEntries(versionResource, []*lockboxEntryCheck{
						{Key: "k1", Regexp: regexp.MustCompile(`arg: , var: , rnd: \d+`)},
						{Key: "k2", Val: "plain value"},
					}),
				),
			},
		},
	})
}

func testAccLockboxSecretVersionBasic(name, secretDesc, versionDesc string) string {
	entries := []*lockboxEntryCheck{
		{Key: "key1", Val: "val1"},
		{Key: "key2", Val: "val2"},
	}
	return testAccLockboxSecretVersion(name, secretDesc, versionDesc, entries)
}

func testAccLockboxSecretVersion(name, secretDesc, versionDesc string, entries []*lockboxEntryCheck) string {
	return fmt.Sprintf(`
resource "yandex_lockbox_secret" "basic_secret" {
  name        = "%v"
  description = "%v"
}

resource "yandex_lockbox_secret_version" "basic_version" {
  secret_id = yandex_lockbox_secret.basic_secret.id
  description = "%v"
  %v
}
`, name, secretDesc, versionDesc, linesForEntries(entries))
}

func linesForEntries(entries []*lockboxEntryCheck) string {
	result := ""
	for _, e := range entries {
		result += lineForEntry(e.Key, e.Val)
	}
	return result
}

func lineForEntry(k string, v string) string {
	return fmt.Sprintf(`
entries {
    key        = "%v"
    text_value = "%v"
}
`, k, v)
}

func testAccLockboxSecretVersionWithCommand(name, cmd, arg, env string) string {
	if arg != "" {
		arg = fmt.Sprintf(`args = ["%s"]`, arg)
	}
	if env != "" {
		env = fmt.Sprintf(`env = { VALUE = "%s" }`, env)
	}
	return fmt.Sprintf(`
resource "yandex_lockbox_secret" "exec_secret" {
  name        = "%v"
}

resource "yandex_lockbox_secret_version" "exec_version" {
  secret_id = yandex_lockbox_secret.exec_secret.id
  entries {
    key = "k1"
    command {
      path = "%v"
      %v
      %v
    }
  }
  entries {
    key = "k2"
    text_value = "plain value"
  }
}
`, name, cmd, arg, env)
}

// Checks expectedEntries in the real secret version of the versionResource
// We can't check entries in state because the resource doesn't read the entries.
func testAccCheckYandexLockboxVersionEntries(versionResource string, expectedEntries []*lockboxEntryCheck) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		config := testAccProvider.Meta().(*Config)
		rs, ok := s.RootModule().Resources[versionResource]
		if !ok {
			return fmt.Errorf("not found resource: %s", versionResource)
		}
		payload, err := config.sdk.LockboxPayload().Payload().Get(context.Background(), &lockbox.GetPayloadRequest{
			SecretId:  rs.Primary.Attributes["secret_id"],
			VersionId: rs.Primary.ID,
		})
		if err != nil {
			return err
		}
		if len(expectedEntries) != len(payload.GetEntries()) {
			return fmt.Errorf("expected %d entries but found %d", len(expectedEntries), len(payload.GetEntries()))
		}
		for i, entry := range payload.GetEntries() {
			expectedEntry := expectedEntries[i]
			if entry.Key != expectedEntry.Key {
				return fmt.Errorf("entry at index %d should have key '%s' but has key '%s'", i, expectedEntry.Key, entry.Key)
			}
			if expectedEntry.Regexp != nil {
				if !expectedEntry.Regexp.MatchString(entry.GetTextValue()) {
					return fmt.Errorf("entry at index %d should have value that matches '%v' but has value '%s'", i, expectedEntry.Regexp, entry.GetTextValue())
				}
			} else {
				if entry.GetTextValue() != expectedEntry.Val {
					return fmt.Errorf("entry at index %d should have value '%s' but has value '%s'", i, expectedEntry.Val, entry.GetTextValue())
				}
			}
		}
		return nil
	}
}
