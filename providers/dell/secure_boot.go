package dell

import (
	"context"

	"github.com/pkg/errors"

	bmclibErrs "github.com/bmc-toolbox/bmclib/v2/errors"
)

// secureBootPolicyAttribute and secureBootAttribute are Dell's vendor-specific BIOS attribute
// names and MUST NOT leak into any exported bmclib identifier.
const (
	secureBootPolicyAttribute = "SecureBootPolicy"
	secureBootPolicyCustom    = "Custom"
	secureBootPolicyStandard  = "Standard"

	secureBootAttribute = "SecureBoot"
)

// SetSecureBoot enables or disables UEFI Secure Boot.
//
// Unlike the Redfish-generic provider, which PATCHes the standard ComputerSystem SecureBoot
// resource's SecureBootEnable property directly, this PATCHes Dell's own SecureBoot BIOS Setup
// attribute instead, through setBiosConfiguration. Confirmed live that PATCHing the SecureBoot
// resource directly still seals the same Dell BIOSConfiguration job as any other BIOS Setup
// attribute change, but without setBiosConfiguration's conflict recovery it fails outright
// (IDRAC.2.14.SYS011) whenever a different pending job already exists, instead of merging with it
// like every other Dell BIOS-backed feature in this package.
//
// Implements bmc.SecureBootSetter.
func (c *Conn) SetSecureBoot(ctx context.Context, enable bool) (err error) {
	want := attrDisabled
	if enable {
		want = attrEnabled
	}

	return c.setBiosConfiguration(ctx, map[string]string{secureBootAttribute: want})
}

// SetSecureBootKeyManagement sets the SecureBootPolicy BIOS attribute to
// Custom or Standard.
//
// This always PATCHes through setBiosConfiguration, even if the attribute's currently *applied*
// value already matches what's requested: currently-applied state can spuriously match while a
// different value is genuinely *pending* from an earlier call in the same boot cycle (e.g. a
// stale pending PATCH left staged by an interrupted, unrelated Workflow) - confirmed live, this
// silently left SecureBootPolicy staged as Standard despite a request to set it to Custom, on the
// SAME class of bug UpdateBiosAttributesExact was written to fix in gofish. Skipping the PATCH
// when currently-applied already matches is exactly the mistake to avoid; setBiosConfiguration's
// own recovery (see recoverFromPendingSettingsConflict) already makes the redundant-PATCH case
// cheap. The write is staged into the Bios/Settings resource and only takes effect on the next
// POST, so a successful call always reports rebootRequired true.
//
// Implements bmc.SecureBootKeyManagementSetter.
func (c *Conn) SetSecureBootKeyManagement(ctx context.Context, enable bool) (rebootRequired bool, err error) {
	biosConfig, err := c.redfishwrapper.GetBiosConfiguration(ctx)
	if err != nil {
		return false, err
	}

	if _, ok := biosConfig[secureBootPolicyAttribute]; !ok {
		return false, bmclibErrs.NewErrUnsupportedHardware(
			secureBootPolicyAttribute + " BIOS attribute not present: platform has no out-of-band Secure Boot key management control",
		)
	}

	want := secureBootPolicyStandard
	if enable {
		want = secureBootPolicyCustom
	}

	if err := c.setBiosConfiguration(ctx, map[string]string{secureBootPolicyAttribute: want}); err != nil {
		return false, errors.Wrapf(err, "failed to set %s", secureBootPolicyAttribute)
	}

	return true, nil
}
