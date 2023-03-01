package main

import (
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/apigee/registry-experimental/pkg/yamlquery"
	"github.com/apigee/registry/pkg/encoding"
	"github.com/timburks/menage/pkg/fetch"
	yaml "gopkg.in/yaml.v3"
)

var dir = "deps"
var top = "twilio-oai"
var provider = "twilio.com"

// List source repos. If a string follows ";", it is the proto path in the repo.
var deps = []string{
	"https://github.com/twilio/twilio-oai",
}
var out = "twilio.com"

func main() {
	err := fetch.FetchDependencies(dir, deps)
	if err != nil {
		panic(err)
	}
	updated := time.Now().Format("2006-01-02")
	commit, err := fetch.CommitHash(filepath.Join(dir, top))
	if err != nil {
		panic(err)
	}
	apis := make(map[string]*encoding.Api)
	filepath.WalkDir(filepath.Join(dir, top, "spec/yaml"),
		func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			} else if entry.IsDir() {
				return nil // Do nothing for the directory, but still walk its contents.
			}
			filename := filepath.Base(path)
			re := regexp.MustCompile(`^twilio_(.*)_(.*)\.yaml$`)
			m := re.FindStringSubmatch(filename)
			if len(m) < 2 {
				return nil
			}
			sourceURI := deps[0] + "/blob/" + commit +
				strings.TrimPrefix(path, filepath.Join(dir, top))
			apiID := strings.ReplaceAll(m[1], "_", "-")
			versionID := m[2]
			bytes, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			var node yaml.Node
			if err := yaml.Unmarshal(bytes, &node); err != nil {
				return err
			}
			openapi := yamlquery.QueryString(&node, "openapi")
			description := yamlquery.QueryString(&node, "info.description")
			if description == nil {
				var empty = ""
				description = &empty
			}
			title := yamlquery.QueryString(&node, "info.title")
			if title == nil {
				var empty = ""
				title = &empty
			}
			if strings.HasSuffix(*title, "Api") {
				*title = strings.ReplaceAll(*title, "Api", "API")
			} else {
				*title += " API"
			}
			*title = strings.ReplaceAll(*title, "Ip_messaging", "IP Messaging")
			*title = strings.ReplaceAll(*title, " - ", " ")
			err = os.MkdirAll(filepath.Join(out, apiID), 0777)
			if err != nil {
				log.Fatalf("%s", err)
			}
			err = os.WriteFile(filepath.Join(out, apiID, filename), bytes, 0666)
			if err != nil {
				log.Fatalf("%s", err)
			}
			api := apis[apiID]
			if api == nil {
				api = &encoding.Api{
					Header: encoding.Header{
						ApiVersion: "apigeeregistry/v1",
						Kind:       "API",
						Metadata: encoding.Metadata{
							Name: provider + "-" + apiID,
							Labels: map[string]string{
								"categories": "telecom",
								"provider":   "twilio-com",
								"updated":    updated,
							},
						},
					},
					Data: encoding.ApiData{
						DisplayName: *title,
						Description: *description,
					},
				}
				apis[apiID] = api
			}
			api.Data.ApiVersions = append(api.Data.ApiVersions,
				&encoding.ApiVersion{
					Header: encoding.Header{
						Metadata: encoding.Metadata{
							Name: versionID,
							Labels: map[string]string{
								"updated": updated,
							},
						},
					},
					Data: encoding.ApiVersionData{
						DisplayName: versionID,
						State:       "production",
						ApiSpecs: []*encoding.ApiSpec{
							{
								Header: encoding.Header{
									Metadata: encoding.Metadata{
										Name: "openapi",
										Labels: map[string]string{
											"updated": updated,
										},
									},
								},
								Data: encoding.ApiSpecData{
									FileName:  filename,
									MimeType:  "application/x.openapi+gzip;version=" + *openapi,
									SourceURI: sourceURI,
								},
							},
						},
					},
				},
			)
			return nil
		})
	for apiID, v := range apis {
		b, err := encoding.EncodeYAML(v)
		if err != nil {
			return
		}
		err = os.WriteFile(filepath.Join(out, apiID, "info.yaml"), b, 0666)
		if err != nil {
			log.Fatalf("%s", err)
		}
	}
}
