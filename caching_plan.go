package gbox

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/caddyserver/caddy/v2"
	"github.com/jensneuse/graphql-go-tools/pkg/astparser"
	"github.com/jensneuse/graphql-go-tools/pkg/astprinter"
	"github.com/jensneuse/graphql-go-tools/pkg/graphql"
	"github.com/jensneuse/graphql-go-tools/pkg/operationreport"
	"github.com/jensneuse/graphql-go-tools/pkg/pool"
	"time"
)

var (
	cachingPlanCacheKeyPattern   = "gbox_cp_%d"
	cachingQueryResultKeyPattern = "gbox_cqr_%d"
)

type cachingPlan struct {
	MaxAge      *caddy.Duration
	Swr         *caddy.Duration
	VaryNames   map[string]struct{}
	Types       map[string]struct{}
	RulesHash   uint64
	VariesHash  uint64
	Passthrough bool

	queryResultCacheKey string
}

type cachingPlanner struct {
	caching       *Caching
	request       *cachingRequest
	schema        *graphql.Schema
	ctxBackground context.Context
	cacheKey      string
}

func newCachingPlanner(r *cachingRequest, c *Caching) (*cachingPlanner, error) {
	hash := pool.Hash64.Get()
	defer pool.Hash64.Put(hash)
	hash.Reset()

	if schemaHash, err := r.schema.Hash(); err != nil {
		return nil, err
	} else {
		hash.Write([]byte(fmt.Sprintf("schema=%d; ", schemaHash)))
	}

	gqlRequestClone := *r.gqlRequest
	documentBuffer := bufferPool.Get().(*bytes.Buffer)
	defer bufferPool.Put(documentBuffer)
	documentBuffer.Reset()

	if _, err := gqlRequestClone.Print(documentBuffer); err != nil {
		return nil, err
	}

	document, _ := astparser.ParseGraphqlDocumentBytes(documentBuffer.Bytes())
	gqlRequestClone.Query, _ = astprinter.PrintString(&document, nil)

	if err := json.NewEncoder(hash).Encode(gqlRequestClone); err != nil {
		return nil, err
	}

	return &cachingPlanner{
		caching:       c,
		request:       r,
		ctxBackground: c.ctxBackground,
		cacheKey:      fmt.Sprintf(cachingPlanCacheKeyPattern, hash.Sum64()),
	}, nil
}

func (c *Caching) getCachingPlan(r *cachingRequest) (plan *cachingPlan, err error) {
	var planner *cachingPlanner
	planner, err = newCachingPlanner(r, c)

	if err != nil {
		return nil, err
	}

	plan, err = planner.getPlan()

	if err != nil {
		return nil, err
	}

	return plan, nil
}

func (p *cachingPlanner) getPlan() (plan *cachingPlan, err error) {
	defer func() {
		if plan == nil {
			return
		}

		var queryResultCacheKey string
		queryResultCacheKey, err = p.calcQueryResultCacheKey(plan)

		if err == nil {
			plan.queryResultCacheKey = queryResultCacheKey
		} else {
			plan = nil
		}
	}()

	if plan, err = p.getCached(); err == nil {
		return plan, nil
	}

	plan, err = p.computePlan()

	if err != nil {
		return nil, err
	}

	if err = p.savePlan(plan); err != nil {
		return nil, err
	}

	return plan, err
}

func (p *cachingPlanner) getCached() (*cachingPlan, error) {
	ctx := p.request.httpRequest.Context()
	cachedPlan := new(cachingPlan)

	if _, err := p.caching.store.Get(ctx, p.cacheKey, cachedPlan); err != nil {
		return nil, err
	}

	if rulesHash, err := p.caching.Rules.hash(); err != nil {
		return nil, err
	} else if rulesHash != cachedPlan.RulesHash {
		return nil, errors.New("invalid checksum rules")
	}

	if variesHash, err := p.caching.Varies.hash(); err != nil {
		return nil, err
	} else if variesHash != cachedPlan.VariesHash {
		return nil, errors.New("invalid checksum varies")
	}

	return cachedPlan, nil
}

func (p *cachingPlanner) savePlan(plan *cachingPlan) error {
	ctx := p.request.httpRequest.Context()

	return p.caching.store.Set(ctx, p.cacheKey, plan, nil)
}

func (p *cachingPlanner) computePlan() (*cachingPlan, error) {
	types := make(map[string]struct{})
	varyNames := make(map[string]struct{})
	plan := &cachingPlan{
		Passthrough: true,
	}

	if rulesHash, err := p.caching.Rules.hash(); err != nil {
		return nil, err
	} else {
		plan.RulesHash = rulesHash
	}

	if variesHash, err := p.caching.Varies.hash(); err != nil {
		return nil, err
	} else {
		plan.VariesHash = variesHash
	}

	requestFieldTypes := make(graphql.RequestTypes)
	extractor := graphql.NewExtractor()
	extractor.ExtractFieldsFromRequest(p.request.gqlRequest, p.request.schema, &operationreport.Report{}, requestFieldTypes)

	for _, rule := range p.caching.Rules {
		if !p.matchRule(requestFieldTypes, rule) {
			continue
		}

		if rule.MaxAge != nil {
			if plan.MaxAge == nil || time.Duration(*plan.MaxAge) > time.Duration(*rule.MaxAge) {
				plan.MaxAge = rule.MaxAge
			}
		}

		if rule.Swr != nil {
			if plan.Swr == nil || time.Duration(*plan.Swr) > time.Duration(*rule.Swr) {
				plan.Swr = rule.Swr
			}
		}

		for vary := range rule.Varies {
			varyNames[vary] = struct{}{}
		}

		if rule.Types == nil {
			types = nil
		} else if types != nil {
			for typeName := range rule.Types {
				types[typeName] = struct{}{}
			}
		}

		plan.Passthrough = false
	}

	plan.VaryNames = varyNames
	plan.Types = types

	return plan, nil
}

func (p *cachingPlanner) matchRule(requestTypes graphql.RequestTypes, rule *CachingRule) bool {
mainLoop:
	for name, fields := range rule.Types {
		compareFields, typeExist := requestTypes[name]

		if !typeExist {
			continue mainLoop
		}

		for field := range fields {
			if _, fieldExist := compareFields[field]; !fieldExist {
				continue mainLoop
			}
		}

		return true
	}

	return rule.Types == nil
}

func (p *cachingPlanner) calcQueryResultCacheKey(plan *cachingPlan) (string, error) {
	hash := pool.Hash64.Get()
	defer pool.Hash64.Put(hash)
	hash.Reset()

	hash.Write([]byte(fmt.Sprintf("%s;", p.cacheKey)))

	r := p.request.httpRequest

	for name := range plan.VaryNames {
		vary, ok := p.caching.Varies[name]

		if !ok {
			return "", fmt.Errorf("setting of vary %s does not exist in varies list given", vary)
		}

		for name := range vary.Headers {
			buffString := fmt.Sprintf("header:%s=%s;", name, r.Header.Get(name))

			if _, err := hash.Write([]byte(buffString)); err != nil {
				return "", err
			}
		}

		for name := range vary.Cookies {
			var value string
			cookie, err := r.Cookie(name)

			if err == nil {
				value = cookie.Value
			}

			buffString := fmt.Sprintf("cookie:%s=%s;", cookie, value)

			if _, err := hash.Write([]byte(buffString)); err != nil {
				return "", err
			}
		}
	}

	return fmt.Sprintf(cachingQueryResultKeyPattern, hash.Sum64()), nil
}
