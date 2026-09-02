package lenovo

import (
	"context"

	"github.com/bmc-toolbox/bmclib/v2/bmc"
)

// GetSecureBoot returns whether UEFI Secure Boot is currently enabled.
//
// Implements bmc.SecureBootStateGetter.
func (c *Conn) GetSecureBoot(ctx context.Context) (enabled bool, err error) {
	return c.redfishwrapper.GetSecureBoot(ctx)
}

// SetSecureBoot enables or disables UEFI Secure Boot.
//
// Implements bmc.SecureBootSetter.
func (c *Conn) SetSecureBoot(ctx context.Context, enable bool) (err error) {
	return c.redfishwrapper.SetSecureBoot(ctx, enable)
}

// ResetSecureBootKeys resets the UEFI Secure Boot key databases.
//
// Implements bmc.SecureBootKeysResetter.
func (c *Conn) ResetSecureBootKeys(ctx context.Context, resetType bmc.ResetSecureBootKeysType) (err error) {
	return c.redfishwrapper.ResetSecureBootKeys(ctx, resetType)
}

// ResetSecureBootDatabaseKeys resets a single UEFI Secure Boot key database.
//
// Implements bmc.SecureBootDatabaseKeysResetter.
func (c *Conn) ResetSecureBootDatabaseKeys(ctx context.Context, database bmc.SecureBootDatabase, resetType bmc.ResetSecureBootDatabaseKeysType) (err error) {
	return c.redfishwrapper.ResetSecureBootDatabaseKeys(ctx, database, resetType)
}

// ImportSecureBootCertificate enrolls a certificate into a single UEFI Secure Boot key database.
//
// Implements bmc.SecureBootCertificateImporter.
func (c *Conn) ImportSecureBootCertificate(ctx context.Context, database bmc.SecureBootDatabase, certificatePEM string) (err error) {
	return c.redfishwrapper.ImportSecureBootCertificate(ctx, database, certificatePEM)
}
