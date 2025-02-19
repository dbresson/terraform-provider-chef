package chef

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"time"

	"github.com/hashicorp/terraform/helper/schema"
	"github.com/hashicorp/terraform/terraform"

	chefc "github.com/go-chef/chef"
)

func Provider() terraform.ResourceProvider {
	return &schema.Provider{
		Schema: map[string]*schema.Schema{
			"server_url": {
				Type:         schema.TypeString,
				Required:     true,
				DefaultFunc:  schema.EnvDefaultFunc("CHEF_SERVER_URL", nil),
				Description:  "URL of the root of the target Chef server or organization.",
				ValidateFunc: validateServerURL,
			},
			"client_name": {
				Type:        schema.TypeString,
				Required:    true,
				DefaultFunc: schema.EnvDefaultFunc("CHEF_CLIENT_NAME", nil),
				Description: "Name of a registered client within the Chef server.",
			},
			"private_key_pem": {
				Type:        schema.TypeString,
				Optional:    true,
				DefaultFunc: providerPrivateKeyEnvDefault,
				Deprecated:  "Please use key_material instead",
				Description: "PEM-formatted private key for client authentication.",
			},
			"key_material": {
				Type:        schema.TypeString,
				Optional:    true,
				DefaultFunc: schema.EnvDefaultFunc("CHEF_KEY_MATERIAL", ""),
			},
			"allow_unverified_ssl": {
				Type:        schema.TypeBool,
				Optional:    true,
				Description: "If set, the Chef client will permit unverifiable SSL certificates.",
			},
		},

		ResourcesMap: map[string]*schema.Resource{
			//"chef_acl":           resourceChefAcl(),
			//"chef_client":        resourceChefClient(),
			//"chef_cookbook":      resourceChefCookbook(),
			"chef_data_bag":      resourceChefDataBag(),
			"chef_data_bag_item": resourceChefDataBagItem(),
			"chef_environment":   resourceChefEnvironment(),
			"chef_client":        resourceChefClient(),
			"chef_node":          resourceChefNode(),
			"chef_role":          resourceChefRole(),
		},

		ConfigureFunc: providerConfigure,
	}
}

func validateServerURL(val interface{}, key string) (warns []string, errs []error) {
	url := val.(string)
	if !strings.HasSuffix(url, "/") {
		errs = append(errs, fmt.Errorf("Chef Server URL %s must end with a slash", url))
	}
	return
}

func providerConfigure(d *schema.ResourceData) (interface{}, error) {
	config := &chefc.Config{
		Name:    d.Get("client_name").(string),
		BaseURL: d.Get("server_url").(string),
		SkipSSL: d.Get("allow_unverified_ssl").(bool),
		Timeout: 10 * time.Second,
	}

	if v, ok := d.GetOk("private_key_pem"); ok {
		config.Key = v.(string)
	}

	if v, ok := d.GetOk("key_material"); ok {
		config.Key = v.(string)
	}

	return chefc.NewClient(config)
}

func providerPrivateKeyEnvDefault() (interface{}, error) {
	if fn := os.Getenv("CHEF_PRIVATE_KEY_FILE"); fn != "" {
		contents, err := ioutil.ReadFile(fn)
		if err != nil {
			return nil, err
		}
		return string(contents), nil
	}

	return nil, nil
}

func jsonStateFunc(value interface{}) string {
	// Parse and re-stringify the JSON to make sure it's always kept
	// in a normalized form.
	in, ok := value.(string)
	if !ok {
		return "null"
	}
	var tmp map[string]interface{}

	// Assuming the value must be valid JSON since it passed okay through
	// our prepareDataBagItemContent function earlier.
	json.Unmarshal([]byte(in), &tmp)

	jsonValue, _ := json.Marshal(&tmp)
	return string(jsonValue)
}

func runListEntryStateFunc(value interface{}) string {
	// Recipes in run lists can either be naked, like "foo", or can
	// be explicitly qualified as "recipe[foo]". Whichever form we use,
	// the server will always normalize to the explicit form,
	// so we'll normalize too and then we won't generate unnecessary
	// diffs when we refresh.
	in := value.(string)
	if !strings.Contains(in, "[") {
		return fmt.Sprintf("recipe[%s]", in)
	}
	return in
}
