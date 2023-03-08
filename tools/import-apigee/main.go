package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	md "github.com/JohannesKaufmann/html-to-markdown"
	"github.com/apigee/registry-experimental/pkg/yamlquery"
	"github.com/apigee/registry/pkg/application/apihub"
	"github.com/apigee/registry/pkg/encoding"
	"gopkg.in/yaml.v3"
)

func main() {
	err := fetch(os.Args[1], os.Args[2])
	if err != nil {
		log.Fatalf("%s", err)
	}
}

type ApiDoc struct {
	SiteId      string `json:"siteId"`
	Title       string `json:"title"`
	Description string `json:"description"`
	ApiId       string `json:"apiId"`
}

type ApiCatalog struct {
	Status  string `json:"status"`
	Message string `json:"message"`
	Data    struct {
		ApiDocs []*ApiDoc `json:"apiDocs"`
	} `json:"data"`
}

func fetch(site, orgName string) error {
	// start by getting the site id
	resp, err := http.Get(fmt.Sprintf("https://%s/siteid", site))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	siteid := string(b)
	// now get the API catalog
	resp, err = http.Get(fmt.Sprintf(
		"https://%s/portals/api/sites/%s/liveportal/apis",
		site, siteid))
	if err != nil {
		return err
	}
	b, err = io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	var a ApiCatalog
	err = json.Unmarshal(b, &a)
	if err != nil {
		return err
	}
	// fetch and save each API.
	for _, api := range a.Data.ApiDocs {
		fmt.Printf("%s\n", api.ApiId)
		err = process(site, orgName, api)
		if err != nil {
			log.Printf("%s %s", api.ApiId, err)
		}
	}
	return nil
}

var empty string

func process(site, orgName string, apiDoc *ApiDoc) error {
	parts := strings.Split(site, ".")
	base := strings.Join(parts[len(parts)-2:], ".")
	docUrl := fmt.Sprintf("https://%s/docs/%s/1/overview",
		site, apiDoc.ApiId)
	specUrl := fmt.Sprintf("https://%s/portals/api/sites/%s/liveportal/apis/%s/download_spec",
		site, apiDoc.SiteId, apiDoc.ApiId)

	os.MkdirAll(filepath.Join(base, apiDoc.ApiId), 0777)
	res, err := http.Get(specUrl)
	if err != nil {
		fmt.Printf("error making http request: %s\n", err)
		os.Exit(1)
	}
	resBody, err := io.ReadAll(res.Body)
	if err != nil {
		panic(err)
	}
	os.WriteFile(filepath.Join(base, apiDoc.ApiId, "openapi.yaml"), resBody, 0666)
	var node yaml.Node
	if err := yaml.Unmarshal(resBody, &node); err != nil {
		panic(err)
	}
	openapi := yamlquery.QueryString(&node, "openapi")
	swagger := yamlquery.QueryString(&node, "swagger")
	var specVersion string
	if openapi != nil {
		specVersion = *openapi
	} else if swagger != nil {
		specVersion = *swagger
	}
	description := yamlquery.QueryString(&node, "info.description")
	if description == nil {
		description = &empty
	}
	*description = markdownify(*description)
	title := yamlquery.QueryString(&node, "info.title")
	if title == nil {
		title = &empty
	}
	referenceData, err := encoding.NodeForMessage(&apihub.ReferenceList{
		DisplayName: "Related Links",
		References: []*apihub.ReferenceList_Reference{
			{
				Id:          "docs",
				DisplayName: "Developer Documentation",
				Uri:         docUrl,
			},
		},
	})
	if err != nil {
		panic(err)
	}
	a := &encoding.Api{
		Header: encoding.Header{
			ApiVersion: "apigeeregistry/v1",
			Kind:       "API",
			Metadata: encoding.Metadata{
				Name: base + "-" + apiDoc.ApiId,
				Labels: map[string]string{
					"provider":   strings.ReplaceAll(base, ".", "-"),
					"categories": "education",
				},
			},
		},
		Data: encoding.ApiData{
			DisplayName: orgName + " " + *title,
			Description: *description,
			ApiVersions: []*encoding.ApiVersion{
				{
					Header: encoding.Header{
						Metadata: encoding.Metadata{
							Name: "v1",
						},
					},
					Data: encoding.ApiVersionData{
						DisplayName: "v1",
						State:       "production",
						ApiSpecs: []*encoding.ApiSpec{
							{
								Header: encoding.Header{
									Metadata: encoding.Metadata{
										Name: "openapi",
									},
								},
								Data: encoding.ApiSpecData{
									FileName: "openapi.yaml",
									MimeType: "application/x.openapi+gzip;version=" + specVersion,
								},
							},
						},
					},
				},
			},
			Artifacts: []*encoding.Artifact{
				{
					Header: encoding.Header{
						Kind: "ReferenceList",
						Metadata: encoding.Metadata{
							Name: "related",
						},
					},
					Data: *referenceData,
				},
			},
		},
	}
	b, err := encoding.EncodeYAML(a)
	if err != nil {
		panic(err)
	}
	os.WriteFile(filepath.Join(base, apiDoc.ApiId, "info.yaml"), b, 0666)
	return nil
}

func markdownify(text string) string {
	converter := md.NewConverter("", true, nil)
	markdown, err := converter.ConvertString(text)
	if err != nil {
		return text
	}
	return markdown
}
