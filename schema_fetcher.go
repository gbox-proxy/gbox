package gbox

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/caddyserver/caddy/v2"
	"github.com/jensneuse/graphql-go-tools/pkg/ast"
	"github.com/jensneuse/graphql-go-tools/pkg/astparser"
	"github.com/jensneuse/graphql-go-tools/pkg/astprinter"
	"github.com/jensneuse/graphql-go-tools/pkg/graphql"
	"github.com/jensneuse/graphql-go-tools/pkg/introspection"
	"go.uber.org/zap"
)

const (
	schemaIntrospectionCacheKey = "gbox_schema_introspection"
)

type schemaChangedHandler func(oldDocument, newDocument *ast.Document, oldSchema, newSchema *graphql.Schema)

// schemaFetcher help to fetch SDL of upstream.
type schemaFetcher struct {
	// Upstream url
	upstream string
	header   http.Header
	interval *caddy.Duration
	timeout  caddy.Duration

	caching         *Caching
	context         context.Context
	logger          *zap.Logger
	schema          *graphql.Schema
	schemaDocument  *ast.Document
	onSchemaChanged schemaChangedHandler
}

func (s *schemaFetcher) Provision(ctx caddy.Context) (err error) {
	introspectionData, _ := s.getCachingIntrospectionData()

	if introspectionData != nil {
		if err = s.fetchByIntrospectionData(introspectionData); err != nil {
			return err
		}
	}

	if err = s.fetch(); err != nil {
		return err
	}

	if s.interval == nil {
		return nil
	}

	go s.startInterval()

	return nil
}

func (s *schemaFetcher) startInterval() {
	interval := time.NewTicker(time.Duration(*s.interval))

	defer interval.Stop()

	for {
		select {
		case <-s.context.Done():
			s.logger.Info("fetch schema interval context cancelled")

			return
		case <-interval.C:
			if err := s.fetch(); err != nil {
				s.logger.Error("interval fetch schema fail", zap.Error(err))
			}
		}
	}
}

func (s *schemaFetcher) fetch() error {
	data, err := s.introspect()
	if err != nil {
		return err
	}

	return s.fetchByIntrospectionData(data)
}

func (s *schemaFetcher) fetchByIntrospectionData(data *introspection.Data) (err error) {
	var newSchema *graphql.Schema
	var document *ast.Document
	dataJSON, _ := json.Marshal(data) // nolint:errchkjson
	converter := &introspection.JsonConverter{}

	if document, err = converter.GraphQLDocument(bytes.NewBuffer(dataJSON)); err != nil {
		return err
	}

	documentOutWriter := bufferPool.Get().(*bytes.Buffer)
	defer bufferPool.Put(documentOutWriter)
	documentOutWriter.Reset()

	if err = astprinter.Print(document, nil, documentOutWriter); err != nil {
		return err
	}

	if newSchema, err = graphql.NewSchemaFromReader(documentOutWriter); err != nil {
		return err
	}

	normalizationResult, _ := newSchema.Normalize()

	if !normalizationResult.Successful {
		return normalizationResult.Errors
	}

	s.schemaChanged(newSchema)

	return nil
}

func (s *schemaFetcher) introspect() (data *introspection.Data, err error) {
	client := &http.Client{
		Timeout: time.Duration(s.timeout),
	}
	requestBody, _ := json.Marshal(s.newIntrospectRequest()) // nolint:errchkjson
	request, _ := http.NewRequestWithContext(s.context, "POST", s.upstream, bytes.NewBuffer(requestBody))
	request.Header = s.header.Clone()
	request.Header.Set("user-agent", "GBox Proxy")
	request.Header.Set("content-type", "application/json")

	var response *http.Response

	response, err = client.Do(request)

	if err != nil {
		return nil, err
	}

	defer response.Body.Close()
	rawResponseBody, _ := ioutil.ReadAll(response.Body)

	var responseBody struct {
		Data *introspection.Data `json:"data"`
	}

	if err = json.Unmarshal(rawResponseBody, &responseBody); err != nil {
		return nil, err
	}

	if responseBody.Data == nil {
		return nil, fmt.Errorf("introspection response not have data field")
	}

	if err = s.cachingIntrospectionData(responseBody.Data); err != nil {
		return nil, err
	}

	return responseBody.Data, nil
}

func (s *schemaFetcher) getCachingIntrospectionData() (*introspection.Data, error) {
	if s.caching == nil {
		return nil, nil // nolint:nilnil
	}

	data := new(introspection.Data)

	if _, err := s.caching.store.Get(s.context, schemaIntrospectionCacheKey, data); err != nil {
		return nil, err
	}

	return data, nil
}

func (s *schemaFetcher) cachingIntrospectionData(data *introspection.Data) error {
	if s.caching == nil {
		return nil
	}

	return s.caching.store.Set(s.context, schemaIntrospectionCacheKey, data, nil)
}

func (s *schemaFetcher) schemaChanged(changedSchema *graphql.Schema) {
	var changedDocument *ast.Document

	defer func() {
		s.schema = changedSchema
		s.schemaDocument = changedDocument
	}()

	if s.onSchemaChanged == nil {
		return
	}

	document, _ := astparser.ParseGraphqlDocumentBytes(changedSchema.Document())
	changedDocument = &document

	if s.schema == nil {
		s.onSchemaChanged(nil, changedDocument, nil, changedSchema)

		return
	}

	oldHash, _ := s.schema.Hash() // nolint:ifshort
	newHash, _ := changedSchema.Hash()

	if oldHash != newHash {
		s.onSchemaChanged(s.schemaDocument, changedDocument, s.schema, changedSchema)
	}
}

func (*schemaFetcher) newIntrospectRequest() *graphql.Request {
	return &graphql.Request{
		OperationName: "IntrospectionQuery",
		Query: `
  query IntrospectionQuery {
    __schema {
      queryType { name }
      mutationType { name }
      subscriptionType { name }
      types {
        ...FullType
      }
      directives {
        name
        args {
          ...InputValue
        }
        locations
      }
    }
  }

  fragment FullType on __Type {
    kind
    name
    fields(includeDeprecated: true) {
      name
      args {
        ...InputValue
      }
      type {
        ...TypeRef
      }
      isDeprecated
      deprecationReason
    }
    inputFields {
      ...InputValue
    }
    interfaces {
      ...TypeRef
    }
    enumValues(includeDeprecated: true) {
      name
      isDeprecated
      deprecationReason
    }
    possibleTypes {
      ...TypeRef
    }
  }

  fragment InputValue on __InputValue {
    name
    type { ...TypeRef }
  }

  fragment TypeRef on __Type {
    kind
    name
    ofType {
      kind
      name
      ofType {
        kind
        name
        ofType {
          kind
          name
          ofType {
            kind
            name
            ofType {
              kind
              name
              ofType {
                kind
                name
                ofType {
                  kind
                  name
                }
              }
            }
          }
        }
      }
    }
  }
`,
	}
}
