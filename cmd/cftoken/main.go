package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"net"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"cftoken/internal/cloudflare"
	"cftoken/internal/config"
)

func main() {
	flags := struct {
		tokenPrefix     string
		zoneID          string
		zoneName        string
		permissions     string
		ttl             time.Duration
		listPermissions bool
		listZones       bool
		allowCIDRs      string
		inspect         bool
		inspectToken    string
		dryRun          bool
		timeout         time.Duration
		verbose         bool
	}{
		timeout: 30 * time.Second,
		verbose: false,
		ttl:     8 * time.Hour,
	}

	const defaultAllowedCIDRList = "10.0.0.1/32,10.0.0.2/32"

	flag.StringVar(&flags.tokenPrefix, "token-prefix", "", "Prefix for the new API token (creation timestamp appended automatically)")
	flag.StringVar(&flags.zoneID, "zone-id", "", "Zone identifier (UUID) the new token should access")
	flag.StringVar(&flags.zoneName, "zone", "", "Zone name present in built-in or configured zone list")
	flag.StringVar(&flags.permissions, "permissions", "", "Comma-separated permission group names or IDs (default: Zone:Read)")
	flag.DurationVar(&flags.ttl, "ttl", flags.ttl, "Token TTL (use 0 for no expiration)")
	flag.BoolVar(&flags.listPermissions, "list-permissions", false, "List permission groups available to the current token and exit")
	flag.BoolVar(&flags.listZones, "list-zones", false, "List configured zones, then exit")
	flag.StringVar(&flags.allowCIDRs, "allow-cidrs", defaultAllowedCIDRList, "Comma-separated CIDRs allowed to use the token")
	flag.BoolVar(&flags.inspect, "inspect", false, "Inspect token details. With token creation this inspects the new token; otherwise it inspects the management token or a provided value.")
	flag.StringVar(&flags.inspectToken, "inspect-token", "", "Token value to inspect when used with -inspect outside of token creation")
	flag.BoolVar(&flags.dryRun, "dry-run", false, "Preview the token creation without calling the Cloudflare API")
	flag.DurationVar(&flags.timeout, "timeout", flags.timeout, "Request timeout (e.g. 15s, 1m)")
	flag.BoolVar(&flags.verbose, "v", flags.verbose, "Enable verbose logging")
	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "Usage: %s [flags]\n\n", os.Args[0])
		fmt.Fprintln(flag.CommandLine.Output(), "Environment:")
		fmt.Fprintln(flag.CommandLine.Output(), "  CLOUDFLARE_API_TOKEN   Cloudflare API token with permission to create tokens (required).")
		fmt.Fprintln(flag.CommandLine.Output())
		flag.PrintDefaults()
	}
	flag.Parse()

	if flag.NArg() == 0 && os.Args != nil && len(os.Args) <= 1 {
		flag.Usage()
		return
	}

	token := strings.TrimSpace(os.Getenv("CLOUDFLARE_API_TOKEN"))
	if token == "" {
		log.Fatalf("missing API token: export CLOUDFLARE_API_TOKEN before running this command")
	}

	ctx, cancel := context.WithTimeout(context.Background(), flags.timeout)
	defer cancel()

	logger := func(string, ...interface{}) {}
	if flags.verbose {
		logger = log.Printf
	}

	client := cloudflare.NewClient(token,
		cloudflare.WithUserAgent("cftoken-cli/0.1"),
		cloudflare.WithLogger(logger),
	)

	if flags.listPermissions {
		perms, err := client.PermissionGroups(ctx)
		if err != nil {
			log.Fatalf("failed to fetch permission groups: %v", err)
		}
		for _, pg := range perms {
			fmt.Printf("%s\t%s\n", pg.ID, pg.Name)
			desc := pg.Description
			if desc == "" {
				desc = pg.Meta.Description
			}
			if desc != "" {
				fmt.Printf("    %s\n", desc)
			}
			if pg.Meta.Key != "" {
				fmt.Printf("    key: %s\n", pg.Meta.Key)
			}
		}
		return
	}

	if flags.listZones {
		zones, err := config.ListConfiguredZones()
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				if path, pathErr := config.ZonesPath(); pathErr == nil {
					log.Fatalf("no zones configured; create %s", path)
				}
				log.Fatalf("no zones configured; create a zones.json file in your config directory")
			}
			log.Fatalf("failed to load configured zones: %v", err)
		}
		tw := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(tw, "ZONE\tID\tSOURCE")
		for _, zone := range zones {
			fmt.Fprintf(tw, "%s\t%s\t%s\n", zone.Name, zone.ID, zone.Source)
		}
		tw.Flush()
		return
	}

	flags.tokenPrefix = strings.TrimSpace(flags.tokenPrefix)
	flags.zoneID = strings.TrimSpace(flags.zoneID)
	flags.zoneName = strings.TrimSpace(flags.zoneName)
	flags.allowCIDRs = strings.TrimSpace(flags.allowCIDRs)
	flags.inspectToken = strings.TrimSpace(flags.inspectToken)

	if flags.inspectToken != "" && !flags.inspect {
		log.Fatalf("-inspect-token requires -inspect")
	}

	createToken := flags.tokenPrefix != ""
	if createToken && flags.inspectToken != "" {
		log.Fatalf("-inspect-token cannot be combined with token creation; the new token is inspected automatically")
	}
	if flags.inspect && !createToken {
		if err := runInspection(ctx, client, flags.inspectToken); err != nil {
			log.Fatalf("inspect token: %v", err)
		}
		return
	}

	if flags.tokenPrefix == "" {
		log.Fatalf("missing token prefix: provide via -token-prefix")
	}

	zoneID := flags.zoneID
	var resolvedZoneName string
	if zoneID == "" && flags.zoneName != "" {
		if id, err := config.ResolveZoneID(flags.zoneName); err == nil {
			zoneID = id
			resolvedZoneName = flags.zoneName
		} else if looksLikeZoneID(flags.zoneName) {
			zoneID = flags.zoneName
		} else {
			log.Fatalf("resolve zone %q: %v", flags.zoneName, err)
		}
	}
	if zoneID == "" {
		log.Fatalf("missing zone identifier: provide via -zone-id or -zone")
	}

	var configuredPermissions []string
	if cfgPerms, err := config.LoadDefaultPermissions(); err == nil {
		configuredPermissions = cfgPerms
	} else if !errors.Is(err, fs.ErrNotExist) {
		log.Fatalf("load default permissions: %v", err)
	}

	var permissionInputs []string
	if strings.TrimSpace(flags.permissions) == "" {
		switch {
		case len(configuredPermissions) > 0:
			permissionInputs = append([]string(nil), configuredPermissions...)
		default:
			permissionInputs = append([]string(nil), cloudflare.DefaultPermissionKeys...)
		}
	} else {
		for _, part := range strings.Split(flags.permissions, ",") {
			if trimmed := strings.TrimSpace(part); trimmed != "" {
				permissionInputs = append(permissionInputs, trimmed)
			}
		}
	}

	creationTime := time.Now().UTC()
	tokenName := flags.tokenPrefix + "-" + creationTime.Format("20060102T150405Z")

	allowedCIDRs, err := parseAllowedCIDRs(flags.allowCIDRs)
	if err != nil {
		log.Fatalf("parse CIDRs: %v", err)
	}
	if len(allowedCIDRs) == 0 {
		log.Fatalf("no allowed CIDRs provided; use -allow-cidrs to specify one or more ranges")
	}

	var expiresOn *time.Time
	if flags.ttl > 0 {
		exp := creationTime.Add(flags.ttl)
		expiresOn = &exp
	}

	if flags.dryRun {
		if err := printDryRun(ctx, client, tokenName, zoneID, resolvedZoneName, permissionInputs, expiresOn, allowedCIDRs); err != nil {
			log.Fatalf("dry run failed: %v", err)
		}
		return
	}

	result, err := client.CreateToken(ctx, tokenName, zoneID, permissionInputs, expiresOn, allowedCIDRs)
	if err != nil {
		log.Fatalf("token creation failed: %v", err)
	}

	printTokenResult(result, resolvedZoneName, flags.ttl)
	if flags.inspect {
		desc, err := client.DescribeToken(ctx, result.ID)
		if err != nil {
			log.Fatalf("inspect token: %v", err)
		}
		printTokenInspection(desc)
	}
}

func looksLikeZoneID(s string) bool {
	if len(s) != 32 {
		return false
	}
	for _, r := range s {
		switch {
		case '0' <= r && r <= '9':
		case 'a' <= r && r <= 'f':
		case 'A' <= r && r <= 'F':
		default:
			return false
		}
	}
	return true
}

func parseAllowedCIDRs(input string) ([]string, error) {
	if strings.TrimSpace(input) == "" {
		return nil, nil
	}
	parts := strings.Split(input, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		trimmed := strings.TrimSpace(p)
		if trimmed == "" {
			continue
		}
		if _, _, err := net.ParseCIDR(trimmed); err != nil {
			return nil, fmt.Errorf("invalid CIDR %q: %w", trimmed, err)
		}
		out = append(out, trimmed)
	}
	return out, nil
}

func printTokenResult(result *cloudflare.TokenResult, zoneName string, ttl time.Duration) {
	fmt.Println("Token created successfully.")
	fmt.Printf("Name:   %s\n", result.Name)
	fmt.Printf("ID:     %s\n", result.ID)
	if result.Value != "" {
		fmt.Printf("Value:  %s\n", result.Value)
	} else {
		fmt.Println("Value:  <redacted by API>")
	}
	fmt.Printf("Status: %s\n", result.Status)
	if zoneName != "" {
		fmt.Printf("Zone ID: %s (%s)\n", result.ZoneID, zoneName)
	} else {
		fmt.Printf("Zone ID: %s\n", result.ZoneID)
	}
	if result.ExpiresOn != "" {
		fmt.Printf("Expires: %s\n", result.ExpiresOn)
	} else if ttl > 0 {
		fmt.Println("Expires: <not returned>")
	} else {
		fmt.Println("Expires: none")
	}
	fmt.Printf("Allowed CIDRs: %s\n", strings.Join(result.AllowedCIDRs, ", "))
}

func printTokenInspection(desc *cloudflare.TokenInspection) {
	if desc == nil {
		fmt.Println("Token details unavailable.")
		return
	}
	fmt.Println("Token details:")
	fmt.Printf("ID: %s\n", desc.ID)
	if desc.Name != "" {
		fmt.Printf("Name: %s\n", desc.Name)
	}
	fmt.Printf("Status: %s\n", desc.Status)
	if desc.ExpiresOn != "" {
		fmt.Printf("Expires: %s\n", desc.ExpiresOn)
	} else {
		fmt.Println("Expires: none")
	}
	if desc.NotBefore != "" {
		fmt.Printf("Not Before: %s\n", desc.NotBefore)
	}
	if len(desc.AllowedCIDRs) > 0 {
		fmt.Printf("Allowed CIDRs: %s\n", strings.Join(desc.AllowedCIDRs, ", "))
	} else {
		fmt.Println("Allowed CIDRs: none")
	}
	if len(desc.DeniedCIDRs) > 0 {
		fmt.Printf("Denied CIDRs: %s\n", strings.Join(desc.DeniedCIDRs, ", "))
	}
	if len(desc.Policies) == 0 {
		fmt.Println("Policies: none")
		return
	}
	fmt.Println("Policies:")
	for idx, policy := range desc.Policies {
		fmt.Printf("  %d. Effect: %s\n", idx+1, policy.Effect)
		if len(policy.Resources) > 0 {
			fmt.Printf("     Resources: %s\n", strings.Join(policy.Resources, ", "))
		}
		if len(policy.PermissionGroups) > 0 {
			fmt.Println("     Permission Groups:")
			for _, grp := range policy.PermissionGroups {
				display := grp.Name
				if display == "" {
					display = grp.Key
				}
				if display == "" {
					display = grp.ID
				}
				if grp.Key != "" && grp.Key != display {
					fmt.Printf("       - %s (%s, key: %s)\n", display, grp.ID, grp.Key)
				} else {
					fmt.Printf("       - %s (%s)\n", display, grp.ID)
				}
			}
		}
	}
}

func runInspection(ctx context.Context, management *cloudflare.Client, overrideToken string) error {
	var (
		verification *cloudflare.TokenVerification
		err          error
	)
	if overrideToken != "" {
		verifyClient := cloudflare.NewClient(overrideToken,
			cloudflare.WithUserAgent("cftoken-cli/0.1"),
		)
		verification, err = verifyClient.VerifyToken(ctx)
		if err != nil {
			return fmt.Errorf("verify token: %w", err)
		}
	} else {
		verification, err = management.VerifyToken(ctx)
		if err != nil {
			return fmt.Errorf("verify token: %w", err)
		}
	}

	desc, err := management.DescribeToken(ctx, verification.ID)
	if err != nil {
		return fmt.Errorf("describe token: %w", err)
	}
	printTokenInspection(desc)
	return nil
}

func printDryRun(ctx context.Context, client *cloudflare.Client, tokenName, zoneID, zoneName string, permissionInputs []string, expiresOn *time.Time, allowedCIDRs []string) error {
	params, matchedGroups, err := client.PreviewToken(ctx, tokenName, zoneID, permissionInputs, expiresOn, allowedCIDRs)
	if err != nil {
		return err
	}

	fmt.Println("DRY RUN: no changes made.")
	fmt.Println("Token would be created with:")
	fmt.Printf("  Name: %s\n", tokenName)
	if zoneName != "" {
		fmt.Printf("  Zone: %s (%s)\n", zoneName, zoneID)
	} else {
		fmt.Printf("  Zone ID: %s\n", zoneID)
	}
	if expiresOn != nil {
		fmt.Printf("  Expires: %s\n", expiresOn.UTC().Format(time.RFC3339))
	} else {
		fmt.Println("  Expires: none")
	}
	if len(allowedCIDRs) > 0 {
		fmt.Printf("  Allowed CIDRs: %s\n", strings.Join(allowedCIDRs, ", "))
	} else {
		fmt.Println("  Allowed CIDRs: none")
	}
	if len(permissionInputs) > 0 {
		fmt.Printf("  Permission inputs: %s\n", strings.Join(permissionInputs, ", "))
	}
	fmt.Println("  Permission groups:")
	if len(matchedGroups) == 0 {
		fmt.Println("    (none)")
	} else {
		for _, group := range matchedGroups {
			display := group.Name
			if display == "" {
				display = group.Meta.Key
			}
			if display == "" {
				display = group.ID
			}
			if group.Meta.Key != "" && group.Meta.Key != display {
				fmt.Printf("    - %s (%s, key: %s)\n", display, group.ID, group.Meta.Key)
			} else {
				fmt.Printf("    - %s (%s)\n", display, group.ID)
			}
		}
	}
	fmt.Println("  Resources:")
	if policiesField := params.Policies; policiesField.Present {
		policies := policiesField.Value
		if len(policies) > 0 {
			fmt.Printf("    - com.cloudflare.api.account.zone.%s -> *\n", zoneID)
		} else {
			fmt.Println("    (none)")
		}
	} else {
		fmt.Println("    (none)")
	}
	return nil
}
