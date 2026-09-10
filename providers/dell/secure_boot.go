package dell

import (
	"context"

	"github.com/pkg/errors"

	bmclibErrs "github.com/bmc-toolbox/bmclib/v2/errors"
)

// secureBootPolicyAttribute is Dell's vendor-specific BIOS attribute name and
// MUST NOT leak into any exported bmclib identifier.
const (
	secureBootPolicyAttribute = "SecureBootPolicy"
	secureBootPolicyCustom    = "Custom"
	secureBootPolicyStandard  = "Standard"
)

// SetSecureBootKeyManagement sets the SecureBootPolicy BIOS attribute to
// Custom or Standard.
//
// The current value is read first so a request matching it is a no-op: scheduling a redundant
// BIOS configuration job is unnecessary, and setBiosConfiguration's recovery from a genuinely
// conflicting in-flight job (see recoverFromPendingSettingsConflict) still costs an extra round
// trip worth skipping when nothing needs to change. The write, when needed, is staged into the
// Bios/Settings resource and only takes effect on the next POST, so a successful change always
// reports rebootRequired true.
//
// Implements bmc.SecureBootKeyManagementSetter.
func (c *Conn) SetSecureBootKeyManagement(ctx context.Context, enable bool) (rebootRequired bool, err error) {
	biosConfig, err := c.redfishwrapper.GetBiosConfiguration(ctx)
	if err != nil {
		return false, err
	}

	current, ok := biosConfig[secureBootPolicyAttribute]
	if !ok {
		return false, bmclibErrs.NewErrUnsupportedHardware(
			secureBootPolicyAttribute + " BIOS attribute not present: platform has no out-of-band Secure Boot key management control",
		)
	}

	want := secureBootPolicyStandard
	if enable {
		want = secureBootPolicyCustom
	}

	if current == want {
		return false, nil
	}

	if err := c.setBiosConfiguration(ctx, map[string]string{secureBootPolicyAttribute: want}); err != nil {
		return false, errors.Wrapf(err, "failed to set %s", secureBootPolicyAttribute)
	}

	return true, nil
}
