package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/apigee/registry/pkg/encoding"
	"github.com/timburks/menage/pkg/extract"
	"github.com/timburks/menage/pkg/fetch"
	yaml "gopkg.in/yaml.v3"
)

var dir = "deps"
var top = "aws-sdk-js"
var provider = "amazon.com"

// List source repos. If a string follows ";", it is the proto path in the repo.
var deps = []string{
	"https://github.com/aws/aws-sdk-js",
}
var out = "amazon.com"

type Collection struct {
	apis map[string]*Api
}

func (c *Collection) getOrCreateApi(name string) *Api {
	if c.apis[name] == nil {
		c.apis[name] = NewApi()
	}
	return c.apis[name]
}

func NewCollection() *Collection {
	return &Collection{apis: make(map[string]*Api)}
}

type Api struct {
	versions map[string]*Version
}

func NewApi() *Api {
	return &Api{versions: make(map[string]*Version)}
}

func (a *Api) getOrCreateVersion(name string) *Version {
	if a.versions[name] == nil {
		a.versions[name] = &Version{parts: make(map[string]string)}
	}
	return a.versions[name]
}

type Version struct {
	parts map[string]string
}

func NewVersion() *Version {
	return &Version{parts: make(map[string]string)}
}

func (v *Version) addPart(name, filename string) {
	v.parts[name] = filename
}

func main() {
	err := fetch.FetchDependencies(dir, deps)
	if err != nil {
		panic(err)
	}

	apis := NewCollection()

	err = filepath.Walk(filepath.Join(dir, top, "apis"),
		func(p string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if info.IsDir() {
				return nil
			}
			if strings.HasSuffix(p, ".json") {
				filename := filepath.Base(p)
				re := regexp.MustCompile(`^(.*)-(\d\d\d\d-\d\d-\d\d).([^.]*).json$`)
				m := re.FindStringSubmatch(filename)
				if len(m) < 4 {
					return nil
				}
				apiID := m[1]
				versionID := m[2]
				part := m[3]
				apis.getOrCreateApi(apiID).getOrCreateVersion(versionID).addPart(part, p)
			}
			return nil
		})
	if err != nil {
		panic(err)
	}
	for k, v := range apis.apis {
		fmt.Printf("%s %d\n", k, len(v.versions))
		for kk, vv := range v.versions {
			path := vv.parts["normal"]
			b, err := os.ReadFile(path)
			if err != nil {
				panic(err)
			}
			var info AmazonNormal
			err = yaml.Unmarshal(b, &info)
			if err != nil {
				panic(err)
			}
			apiID := strings.ToLower(k)
			versionID := kk
			specID := "awsconfig"
			initial := "aws/" + strings.ToLower(apiID[0:1])
			// Build and save info.yaml for the API.
			api := &encoding.Api{
				Header: encoding.Header{
					ApiVersion: encoding.RegistryV1,
					Kind:       "API",
					Metadata: encoding.Metadata{
						Name: provider + "-" + apiID,
						Labels: map[string]string{
							"provider": strings.ReplaceAll(provider, ".", "-"),
						},
					},
				},
				Data: encoding.ApiData{
					DisplayName: displayName(info.Metadata.ServiceFullName),
					Description: extract.Markdownify(info.Documentation),
				},
			}
			b, err = encoding.EncodeYAML(api)
			if err != nil {
				panic(err)
			}
			err = os.MkdirAll(filepath.Join(out, initial, apiID, versionID, specID), 0777)
			if err != nil {
				panic(err)
			}
			name := filepath.Join(out, initial, apiID, "info.yaml")
			err = os.WriteFile(name, b, 0664)
			if err != nil {
				panic(err)
			}

			// Build and save info.yaml for the version.
			apiVersion := &encoding.ApiVersion{
				Header: encoding.Header{
					ApiVersion: encoding.RegistryV1,
					Kind:       "Version",
					Metadata: encoding.Metadata{
						Parent: "apis/" + provider + "-" + apiID,
						Name:   versionID,
					},
				},
				Data: encoding.ApiVersionData{
					DisplayName: versionID,
				},
			}
			b, err = encoding.EncodeYAML(apiVersion)
			if err != nil {
				panic(err)
			}
			name = filepath.Join(out, initial, apiID, versionID, "info.yaml")
			err = os.WriteFile(name, b, 0664)
			if err != nil {
				panic(err)
			}
			// Build and save info.yaml for the spec.
			apiSpec := &encoding.ApiSpec{
				Header: encoding.Header{
					ApiVersion: encoding.RegistryV1,
					Kind:       "Spec",
					Metadata: encoding.Metadata{
						Parent:      "apis/" + provider + "-" + apiID + "/versions/" + versionID,
						Name:        specID,
						Annotations: map[string]string{},
					},
				},
				Data: encoding.ApiSpecData{
					FileName: "awsconfig.zip",
					MimeType: "application/x.awsconfig+zip",
				},
			}
			b, err = encoding.EncodeYAML(apiSpec)
			if err != nil {
				panic(err)
			}
			name = filepath.Join(out, initial, apiID, versionID, specID, "info.yaml")
			err = os.WriteFile(name, b, 0664)
			if err != nil {
				panic(err)
			}
			for _, vvv := range vv.parts {
				b, err := os.ReadFile(vvv)
				if err != nil {
					panic(err)
				}
				name = filepath.Join(out, initial, apiID, versionID, specID, filepath.Base(vvv))
				err = os.WriteFile(name, b, 0664)
				if err != nil {
					panic(err)
				}
			}
		}
	}
}

func displayName(name string) string {
	if !strings.HasPrefix(name, "Amazon") {
		return "Amazon " + name
	}
	return name
}

type AmazonNormal struct {
	Version  string `yaml:"version"`
	Metadata struct {
		ApiVersion       string `yaml:"apiVersion"`
		EndpointPrefix   string `yaml:"endpointPrefix"`
		JsonVersion      string `yaml:"jsonVersion"`
		Protocol         string `yaml:"protocol"`
		ServiceFullName  string `yaml:"serviceFullName"`
		ServiceId        string `yaml:"serviceId"`
		SignatureVersion string `yaml:"signatureVersion"`
		SigningName      string `yaml:"signingName"`
		Uid              string `yaml:"uid"`
	} `yaml:"metadata"`
	Documentation string `yaml:"documentation"`
}
