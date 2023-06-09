package gbox

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/jensneuse/graphql-go-tools/pkg/astvisitor"
	"github.com/jensneuse/graphql-go-tools/pkg/graphql"
	"github.com/jensneuse/graphql-go-tools/pkg/operationreport"
)

const (
	cachingTagSchemaHashPrefix  = "schema:"
	cachingTagSchemaHashPattern = cachingTagSchemaHashPrefix + "%d"
	cachingTagTypeFieldPrefix   = "field:"
	cachingTagTypeFieldPattern  = cachingTagTypeFieldPrefix + "%s:%s"
	cachingTagTypePrefix        = "type:"
	cachingTagTypePattern       = cachingTagTypePrefix + "%s"
	cachingTagTypeKeyPrefix     = "key:"
	cachingTagTypeKeyPattern    = cachingTagTypeKeyPrefix + "%s:%s:%s"
	cachingTagOperationPrefix   = "operation:"
	cachingTagOperationPattern  = cachingTagOperationPrefix + "%s"
)

type cachingTagVisitor struct {
	*cachingTagAnalyzer
	*astvisitor.Walker
	data      map[string]interface{}
	tags      cachingTags
	onlyTypes map[string]struct{}
}

func (c *cachingTagVisitor) EnterField(ref int) {
	operation, definition := c.request.operation, c.request.definition
	fieldName := operation.FieldNameString(ref)
	typeName := definition.NodeNameString(c.EnclosingTypeDefinition)

	if c.onlyTypes != nil && typeName != c.request.schema.QueryTypeName() && typeName != c.request.schema.MutationTypeName() {
		if _, exist := c.onlyTypes[typeName]; !exist {
			return
		}
	}

	c.addTagForType(typeName)
	c.addTagForTypeField(typeName, fieldName)

	keys, ok := c.typeKeys[typeName]

	if !ok {
		keys = graphql.RequestFields{
			"id": struct{}{},
		}
	}

	path := make([]string, 0)

	for _, p := range c.Path[1:] {
		path = append(path, p.FieldName.String())
	}

	path = append(path, operation.FieldAliasOrNameString(ref))

	for key := range keys {
		if key == fieldName {
			c.collectTypeKeyTags(path, c.data, typeName)

			break
		}
	}
}

func (c *cachingTagVisitor) addTagForTypeField(typeName, fieldName string) {
	c.tags[fmt.Sprintf(cachingTagTypeFieldPattern, typeName, fieldName)] = struct{}{}
}

func (c *cachingTagVisitor) addTagForType(typeName string) {
	c.tags[fmt.Sprintf(cachingTagTypePattern, typeName)] = struct{}{}
}

func (c *cachingTagVisitor) addTagForTypeKey(field string, value interface{}, typeName string) {
	switch v := value.(type) {
	case string:
		c.tags[fmt.Sprintf(cachingTagTypeKeyPattern, typeName, field, v)] = struct{}{}
	case float64:
		c.tags[fmt.Sprintf(cachingTagTypeKeyPattern, typeName, field, strconv.FormatInt(int64(v), 10))] = struct{}{}
	default:
		c.Walker.StopWithInternalErr(fmt.Errorf("invalid type key of %s.%s only accept string or numeric but got: %T", typeName, field, v))
	}
}

func (c *cachingTagVisitor) collectTypeKeyTags(path []string, data interface{}, typeName string) {
	at := path[0]

	if next := path[1:]; len(next) > 0 {
		switch v := data.(type) {
		case []interface{}:
			for _, item := range v {
				c.collectTypeKeyTags(path, item, typeName)
			}
		case map[string]interface{}:
			if item, ok := v[at]; ok {
				c.collectTypeKeyTags(next, item, typeName)
			}
		default:
			// skip in cases field value's null
		}

		return
	}

	switch v := data.(type) {
	case []interface{}:
		for _, item := range v {
			c.collectTypeKeyTags(path, item, typeName)
		}
	case map[string]interface{}:
		if v[at] != nil {
			c.addTagForTypeKey(at, v[at], typeName)
		}
	default:
		if v != nil {
			c.Walker.StopWithInternalErr(fmt.Errorf("invalid data type expected map or array map but got %T", v))
		}
	}
}

type cachingTags map[string]struct{}

type cachingTagAnalyzer struct {
	request  *cachingRequest
	typeKeys graphql.RequestTypes
}

func newCachingTagAnalyzer(r *cachingRequest, t graphql.RequestTypes) *cachingTagAnalyzer {
	return &cachingTagAnalyzer{r, t}
}

func (c *cachingTagAnalyzer) AnalyzeResult(result []byte, onlyTypes map[string]struct{}, tags cachingTags) (err error) {
	normalizedQueryResult := &struct {
		Data map[string]interface{} `json:"data,omitempty"`
	}{}

	if err = json.Unmarshal(result, normalizedQueryResult); err != nil {
		return err
	}

	if normalizedQueryResult.Data == nil || len(normalizedQueryResult.Data) == 0 {
		return errors.New("query result: `data` field missing")
	}

	if err = c.request.initOperation(); err != nil {
		return err
	}

	report := &operationreport.Report{}
	walker := astvisitor.NewWalker(48)
	visitor := &cachingTagVisitor{
		cachingTagAnalyzer: c,
		Walker:             &walker,
		data:               normalizedQueryResult.Data,
		tags:               tags,
		onlyTypes:          onlyTypes,
	}

	walker.RegisterEnterFieldVisitor(visitor)
	walker.Walk(c.request.operation, c.request.definition, report)

	if report.HasErrors() {
		return report
	}

	schemaHash, _ := c.request.schema.Hash()
	schemaHashTag := fmt.Sprintf(cachingTagSchemaHashPattern, schemaHash)
	operationTag := fmt.Sprintf(cachingTagOperationPattern, c.request.gqlRequest.OperationName)
	tags[schemaHashTag] = struct{}{}
	tags[operationTag] = struct{}{}

	return nil
}

func (t cachingTags) ToSlice() []string {
	s := make([]string, 0)

	for item := range t {
		s = append(s, item)
	}

	sort.Strings(s)

	return s
}

func (t cachingTags) TypeKeys() cachingTags {
	return t.filterWithPrefix(cachingTagTypeKeyPrefix)
}

func (t cachingTags) Types() cachingTags {
	return t.filterWithPrefix(cachingTagTypePrefix)
}

func (t cachingTags) TypeFields() cachingTags {
	return t.filterWithPrefix(cachingTagTypeFieldPrefix)
}

func (t cachingTags) SchemaHash() cachingTags {
	return t.filterWithPrefix(cachingTagSchemaHashPrefix)
}

func (t cachingTags) Operation() cachingTags {
	return t.filterWithPrefix(cachingTagOperationPrefix)
}

func (t cachingTags) filterWithPrefix(prefix string) cachingTags {
	keys := make(cachingTags)

	for tag := range t {
		if strings.HasPrefix(tag, prefix) {
			keys[tag] = struct{}{}
		}
	}

	return keys
}
