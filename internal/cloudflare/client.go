package cloudflare

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	cf "github.com/cloudflare/cloudflare-go/v6"
	cfoption "github.com/cloudflare/cloudflare-go/v6/option"
	"github.com/cloudflare/cloudflare-go/v6/shared"
	cfuser "github.com/cloudflare/cloudflare-go/v6/user"
)

// DefaultPermissionKeys represents the fallback permission group names used when
// the CLI is not provided explicit permissions. These should correspond to the
// human-readable names returned by the Cloudflare API.
var DefaultPermissionKeys = []string{"Zone:Read"}

// Client wraps the Cloudflare SDK client with helpers needed for token
// provisioning.
type Client struct {
	api        *cf.Client
	baseURL    string
	userAgent  string
	httpClient *http.Client
	logf       func(string, ...interface{})
}

// Option configures a Client.
type Option func(*Client)

// WithBaseURL overrides the Cloudflare API base URL (useful for testing).
func WithBaseURL(baseURL string) Option {
	return func(c *Client) {
		c.baseURL = strings.TrimRight(baseURL, "/")
	}
}

// WithUserAgent overrides the User-Agent header sent on each request.
func WithUserAgent(ua string) Option {
	return func(c *Client) {
		c.userAgent = ua
	}
}

// WithHTTPClient overrides the underlying http.Client.
func WithHTTPClient(client *http.Client) Option {
	return func(c *Client) {
		c.httpClient = client
	}
}

// WithLogger allows wiring in a logger for verbose output.
func WithLogger(logf func(string, ...interface{})) Option {
	return func(c *Client) {
		if logf != nil {
			c.logf = logf
		}
	}
}

// NewClient constructs a Client backed by the official Cloudflare SDK.
func NewClient(token string, opts ...Option) *Client {
	c := &Client{
		userAgent:  "cftoken-cli",
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
	for _, opt := range opts {
		opt(c)
	}

	var requestOptions []cfoption.RequestOption
	requestOptions = append(requestOptions, cfoption.WithAPIToken(token))

	if c.baseURL != "" {
		requestOptions = append(requestOptions, cfoption.WithBaseURL(c.baseURL))
	}
	if c.userAgent != "" {
		requestOptions = append(requestOptions, cfoption.WithHeader("User-Agent", c.userAgent))
	}
	if c.httpClient != nil {
		requestOptions = append(requestOptions, cfoption.WithHTTPClient(c.httpClient))
	}
	if c.logf != nil {
		requestOptions = append(requestOptions, cfoption.WithMiddleware(func(req *http.Request, next cfoption.MiddlewareNext) (*http.Response, error) {
			c.logf("cloudflare request: %s %s", req.Method, req.URL.String())
			return next(req)
		}))
	}

	c.api = cf.NewClient(requestOptions...)
	return c
}

// PermissionGroup describes a permission group that can be attached to an API token.
type PermissionGroup struct {
	ID          string              `json:"id"`
	Name        string              `json:"name"`
	Description string              `json:"description"`
	Scopes      []string            `json:"scopes"`
	Meta        PermissionGroupMeta `json:"meta"`
}

// PermissionGroupMeta captures additional metadata for a permission group.
type PermissionGroupMeta struct {
	Key         string `json:"key"`
	Scope       string `json:"scope"`
	Kind        string `json:"kind"`
	Description string `json:"description"`
}

// TokenResult captures the subset of the create token response we care about.
type TokenResult struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Status       string `json:"status"`
	Value        string `json:"value"`
	ExpiresOn    string `json:"expires_on"`
	ZoneID       string
	AllowedCIDRs []string
}

// TokenVerification captures metadata returned by the verify endpoint.
type TokenVerification struct {
	ID        string
	Status    string
	ExpiresOn string
	NotBefore string
}

// TokenInspection summarises a token's configuration.
type TokenInspection struct {
	ID           string
	Name         string
	Status       string
	ExpiresOn    string
	NotBefore    string
	AllowedCIDRs []string
	DeniedCIDRs  []string
	Policies     []TokenPolicyInspection
}

// TokenPolicyInspection captures the essential components of a token policy.
type TokenPolicyInspection struct {
	Effect           string
	PermissionGroups []PermissionGroupSummary
	Resources        []string
}

// PermissionGroupSummary exposes concise metadata for a permission group.
type PermissionGroupSummary struct {
	ID   string
	Name string
	Key  string
}

// PermissionGroups fetches all permission groups available to the current token.
func (c *Client) PermissionGroups(ctx context.Context) ([]PermissionGroup, error) {
	page, err := c.api.User.Tokens.PermissionGroups.List(ctx, cfuser.TokenPermissionGroupListParams{})
	if err != nil {
		return nil, fmt.Errorf("list permission groups: %w", err)
	}
	if page == nil {
		return nil, errors.New("cloudflare API returned an empty permission group response")
	}

	items := page.Result
	groups := make([]PermissionGroup, 0, len(items))
	for _, item := range items {
		group := PermissionGroup{
			ID:   item.ID,
			Name: item.Name,
		}
		for _, scope := range item.Scopes {
			group.Scopes = append(group.Scopes, string(scope))
		}
		if field, ok := item.JSON.ExtraFields["description"]; ok && !field.IsMissing() && !field.IsNull() && !field.IsInvalid() {
			var desc string
			if err := json.Unmarshal([]byte(field.Raw()), &desc); err == nil {
				group.Description = desc
			}
		}
		if field, ok := item.JSON.ExtraFields["meta"]; ok && !field.IsMissing() && !field.IsNull() && !field.IsInvalid() {
			var meta PermissionGroupMeta
			if err := json.Unmarshal([]byte(field.Raw()), &meta); err == nil {
				group.Meta = meta
			}
		}
		groups = append(groups, group)
	}
	return groups, nil
}

// Policy represents a Cloudflare API token policy ready to be converted to API parameters.
type Policy struct {
	ID               string                 `json:"id,omitempty"`
	Effect           string                 `json:"effect"`
	Resources        map[string]interface{} `json:"resources"`
	PermissionGroups []PolicyPermissionGroup `json:"permission_groups"`
}

// PolicyPermissionGroup represents a permission group in a policy.
type PolicyPermissionGroup struct {
	ID   string `json:"id"`
	Name string `json:"name,omitempty"`
}

// CreateToken provisions a new token scoped to the provided zone identifier with the desired permissions.
func (c *Client) CreateToken(ctx context.Context, tokenName, zoneID string, permissionInputs []string, expiresOn *time.Time, allowedCIDRs []string) (*TokenResult, error) {
	perms, err := c.PermissionGroups(ctx)
	if err != nil {
		return nil, fmt.Errorf("fetch permission groups: %w", err)
	}

	params, _, err := buildTokenParams(perms, tokenName, zoneID, permissionInputs, expiresOn, allowedCIDRs)
	if err != nil {
		return nil, err
	}

	resp, err := c.api.User.Tokens.New(ctx, *params)
	if err != nil {
		return nil, fmt.Errorf("create token: %w", err)
	}
	if resp == nil {
		return nil, errors.New("cloudflare API returned an empty response")
	}

	result := &TokenResult{
		ID:           resp.ID,
		Name:         resp.Name,
		Status:       string(resp.Status),
		Value:        string(resp.Value),
		ZoneID:       zoneID,
		AllowedCIDRs: append([]string(nil), allowedCIDRs...),
	}
	if !resp.ExpiresOn.IsZero() {
		result.ExpiresOn = resp.ExpiresOn.UTC().Format(time.RFC3339)
	}
	return result, nil
}

// CreateTokenWithPolicies provisions a new token using pre-built policy structures from templates.
func (c *Client) CreateTokenWithPolicies(ctx context.Context, tokenName string, policies []Policy, expiresOn *time.Time, allowedCIDRs []string) (*TokenResult, error) {
	params, err := buildTokenParamsFromPolicies(tokenName, policies, expiresOn, allowedCIDRs)
	if err != nil {
		return nil, err
	}

	resp, err := c.api.User.Tokens.New(ctx, *params)
	if err != nil {
		return nil, fmt.Errorf("create token: %w", err)
	}
	if resp == nil {
		return nil, errors.New("cloudflare API returned an empty response")
	}

	result := &TokenResult{
		ID:           resp.ID,
		Name:         resp.Name,
		Status:       string(resp.Status),
		Value:        string(resp.Value),
		AllowedCIDRs: append([]string(nil), allowedCIDRs...),
	}
	if !resp.ExpiresOn.IsZero() {
		result.ExpiresOn = resp.ExpiresOn.UTC().Format(time.RFC3339)
	}
	return result, nil
}

// PreviewToken prepares the payload that would be sent to create a token without executing the API call.
func (c *Client) PreviewToken(ctx context.Context, tokenName, zoneID string, permissionInputs []string, expiresOn *time.Time, allowedCIDRs []string) (*cfuser.TokenNewParams, []PermissionGroup, error) {
	perms, err := c.PermissionGroups(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("fetch permission groups: %w", err)
	}

	params, matchedGroups, err := buildTokenParams(perms, tokenName, zoneID, permissionInputs, expiresOn, allowedCIDRs)
	if err != nil {
		return nil, nil, err
	}
	return params, matchedGroups, nil
}

func buildTokenParams(perms []PermissionGroup, tokenName, zoneID string, permissionInputs []string, expiresOn *time.Time, allowedCIDRs []string) (*cfuser.TokenNewParams, []PermissionGroup, error) {
	permissionRefs, matchedGroups, err := matchPermissionGroups(perms, permissionInputs)
	if err != nil {
		return nil, nil, err
	}

	resourceKey := fmt.Sprintf("com.cloudflare.api.account.zone.%s", zoneID)
	policy := shared.TokenPolicyParam{
		Effect:           cf.F(shared.TokenPolicyEffectAllow),
		PermissionGroups: cf.F(permissionRefs),
		Resources: cf.F[shared.TokenPolicyResourcesUnionParam](
			shared.TokenPolicyResourcesIAMResourcesTypeObjectStringParam{
				resourceKey: "*",
			},
		),
	}

	params := &cfuser.TokenNewParams{
		Name:     cf.F(tokenName),
		Policies: cf.F([]shared.TokenPolicyParam{policy}),
	}
	if expiresOn != nil {
		params.ExpiresOn = cf.F(expiresOn.UTC())
	}
	if len(allowedCIDRs) > 0 {
		values := make([]shared.TokenConditionCIDRListParam, 0, len(allowedCIDRs))
		for _, cidr := range allowedCIDRs {
			values = append(values, shared.TokenConditionCIDRListParam(cidr))
		}
		params.Condition = cf.F(cfuser.TokenNewParamsCondition{
			RequestIP: cf.F(cfuser.TokenNewParamsConditionRequestIP{
				In: cf.F(values),
			}),
		})
	}

	return params, matchedGroups, nil
}

func buildTokenParamsFromPolicies(tokenName string, policies []Policy, expiresOn *time.Time, allowedCIDRs []string) (*cfuser.TokenNewParams, error) {
	if len(policies) == 0 {
		return nil, errors.New("at least one policy is required")
	}

	policyParams := make([]shared.TokenPolicyParam, 0, len(policies))
	for _, policy := range policies {
		// Convert permission groups
		permGroups := make([]shared.TokenPolicyPermissionGroupParam, 0, len(policy.PermissionGroups))
		for _, pg := range policy.PermissionGroups {
			permGroups = append(permGroups, shared.TokenPolicyPermissionGroupParam{
				ID: cf.F(pg.ID),
			})
		}

		// Convert resources map to API format
		resourcesParam := shared.TokenPolicyResourcesIAMResourcesTypeObjectStringParam{}
		for key, value := range policy.Resources {
			if strValue, ok := value.(string); ok {
				resourcesParam[key] = strValue
			} else {
				// Handle non-string values by converting to string
				resourcesParam[key] = fmt.Sprintf("%v", value)
			}
		}

		// Build policy param
		policyParam := shared.TokenPolicyParam{
			PermissionGroups: cf.F(permGroups),
			Resources: cf.F[shared.TokenPolicyResourcesUnionParam](resourcesParam),
		}

		// Set effect (default to "allow" if not specified)
		effect := policy.Effect
		if effect == "" {
			effect = "allow"
		}
		if effect == "allow" {
			policyParam.Effect = cf.F(shared.TokenPolicyEffectAllow)
		} else if effect == "deny" {
			policyParam.Effect = cf.F(shared.TokenPolicyEffectDeny)
		} else {
			return nil, fmt.Errorf("invalid policy effect %q; must be 'allow' or 'deny'", effect)
		}

		policyParams = append(policyParams, policyParam)
	}

	params := &cfuser.TokenNewParams{
		Name:     cf.F(tokenName),
		Policies: cf.F(policyParams),
	}
	if expiresOn != nil {
		params.ExpiresOn = cf.F(expiresOn.UTC())
	}
	if len(allowedCIDRs) > 0 {
		values := make([]shared.TokenConditionCIDRListParam, 0, len(allowedCIDRs))
		for _, cidr := range allowedCIDRs {
			values = append(values, shared.TokenConditionCIDRListParam(cidr))
		}
		params.Condition = cf.F(cfuser.TokenNewParamsCondition{
			RequestIP: cf.F(cfuser.TokenNewParamsConditionRequestIP{
				In: cf.F(values),
			}),
		})
	}

	return params, nil
}

// VerifyToken returns metadata about the token configured on this client.
func (c *Client) VerifyToken(ctx context.Context) (*TokenVerification, error) {
	resp, err := c.api.User.Tokens.Verify(ctx)
	if err != nil {
		return nil, fmt.Errorf("verify token: %w", err)
	}
	if resp == nil {
		return nil, errors.New("cloudflare API returned an empty token verification response")
	}
	out := &TokenVerification{
		ID:     resp.ID,
		Status: string(resp.Status),
	}
	if !resp.ExpiresOn.IsZero() {
		out.ExpiresOn = resp.ExpiresOn.UTC().Format(time.RFC3339)
	}
	if !resp.NotBefore.IsZero() {
		out.NotBefore = resp.NotBefore.UTC().Format(time.RFC3339)
	}
	return out, nil
}

// DescribeToken fetches a token by ID and extracts its permissions and restrictions.
func (c *Client) DescribeToken(ctx context.Context, tokenID string) (*TokenInspection, error) {
	if strings.TrimSpace(tokenID) == "" {
		return nil, errors.New("token ID is required")
	}
	token, err := c.api.User.Tokens.Get(ctx, tokenID)
	if err != nil {
		return nil, fmt.Errorf("get token %s: %w", tokenID, err)
	}
	if token == nil {
		return nil, errors.New("cloudflare API returned an empty token response")
	}

	inspection := &TokenInspection{
		ID:     token.ID,
		Name:   token.Name,
		Status: string(token.Status),
	}
	if !token.ExpiresOn.IsZero() {
		inspection.ExpiresOn = token.ExpiresOn.UTC().Format(time.RFC3339)
	}
	if !token.NotBefore.IsZero() {
		inspection.NotBefore = token.NotBefore.UTC().Format(time.RFC3339)
	}

	for _, cidr := range token.Condition.RequestIP.In {
		inspection.AllowedCIDRs = append(inspection.AllowedCIDRs, string(cidr))
	}
	for _, cidr := range token.Condition.RequestIP.NotIn {
		inspection.DeniedCIDRs = append(inspection.DeniedCIDRs, string(cidr))
	}
	sort.Strings(inspection.AllowedCIDRs)
	sort.Strings(inspection.DeniedCIDRs)

	for _, pol := range token.Policies {
		policy := TokenPolicyInspection{
			Effect: string(pol.Effect),
		}
		policy.PermissionGroups = append(policy.PermissionGroups, summarisePermissionGroups(pol.PermissionGroups)...)
		policy.Resources = extractPolicyResources(pol.Resources)
		sort.Strings(policy.Resources)
		inspection.Policies = append(inspection.Policies, policy)
	}

	return inspection, nil
}

func matchPermissionGroups(groups []PermissionGroup, inputs []string) ([]shared.TokenPolicyPermissionGroupParam, []PermissionGroup, error) {
	if len(inputs) == 0 {
		return nil, nil, errors.New("no permission groups specified")
	}
	matched := make([]shared.TokenPolicyPermissionGroupParam, 0, len(inputs))
	matchedGroups := make([]PermissionGroup, 0, len(inputs))
lookup:
	for _, in := range inputs {
		normalized := normalizeKey(in)
		for _, group := range groups {
			switch {
			case strings.EqualFold(in, group.ID):
				matched = append(matched, shared.TokenPolicyPermissionGroupParam{
					ID: cf.F(group.ID),
				})
				matchedGroups = append(matchedGroups, group)
				continue lookup
			case normalizeKey(group.Name) == normalized:
				matched = append(matched, shared.TokenPolicyPermissionGroupParam{
					ID: cf.F(group.ID),
				})
				matchedGroups = append(matchedGroups, group)
				continue lookup
			case group.Meta.Key != "" && normalizeKey(group.Meta.Key) == normalized:
				matched = append(matched, shared.TokenPolicyPermissionGroupParam{
					ID: cf.F(group.ID),
				})
				matchedGroups = append(matchedGroups, group)
				continue lookup
			}
		}
		return nil, nil, fmt.Errorf("permission group %q not found; rerun with -list-permissions to inspect available values", in)
	}
	return matched, matchedGroups, nil
}

func normalizeKey(s string) string {
	s = strings.TrimSpace(strings.ToLower(s))
	replacer := strings.NewReplacer(" ", "", "_", "", "-", "", ":", "", ".", "")
	return replacer.Replace(s)
}

func summarisePermissionGroups(groups []shared.TokenPolicyPermissionGroup) []PermissionGroupSummary {
	out := make([]PermissionGroupSummary, 0, len(groups))
	for _, group := range groups {
		out = append(out, PermissionGroupSummary{
			ID:   group.ID,
			Name: group.Name,
			Key:  group.Meta.Key,
		})
	}
	return out
}

func extractPolicyResources(res shared.TokenPolicyResourcesUnion) []string {
	switch v := res.(type) {
	case shared.TokenPolicyResourcesIAMResourcesTypeObjectString:
		list := make([]string, 0, len(v))
		for key, value := range v {
			if value == "" {
				list = append(list, key)
				continue
			}
			list = append(list, fmt.Sprintf("%s=%s", key, value))
		}
		return list
	case shared.TokenPolicyResourcesIAMResourcesTypeObjectNested:
		list := make([]string, 0)
		for prefix, nested := range v {
			if len(nested) == 0 {
				list = append(list, prefix)
				continue
			}
			for key, value := range nested {
				list = append(list, fmt.Sprintf("%s.%s=%s", prefix, key, value))
			}
		}
		return list
	default:
		return nil
	}
}
