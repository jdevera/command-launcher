package vault

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/jdevera/command-launcher/internal/context"
)

type Dico map[string]string

type FileVault struct {
	Name string
	hash []byte
}

func (fv *FileVault) Write(key string, value string) error {
	vaultDir, err := maybeCreateDir()
	if err != nil {
		return err
	}

	dico, err := fv.readFile()
	if err != nil {
		return err
	}

	dico[key] = value
	data, err := json.Marshal(dico)
	if err != nil {
		return err
	}

	encrypted, err := fv.encrypt(data)
	if err != nil {
		return err
	}

	return ioutil.WriteFile(filepath.Join(vaultDir, fv.Name), encrypted, 0600)
}

func (fv *FileVault) Read(key string) (string, error) {
	dico, err := fv.readFile()
	if err != nil {
		return "", err
	}

	if len(dico) == 0 {
		return "", fmt.Errorf("vault %s is empty", fv.Name)
	}

	return dico[key], nil
}

func (fv *FileVault) readFile() (Dico, error) {
	dico := make(Dico)
	vaultDir, err := maybeCreateDir()
	if err != nil {
		return dico, err
	}

	encrypted, err := ioutil.ReadFile(filepath.Join(vaultDir, fv.Name))
	if err != nil {
		return dico, err
	}

	if len(encrypted) == 0 {
		return dico, err
	}

	data, err := fv.decrypt(encrypted)
	if err != nil {
		return dico, err
	}

	err = json.Unmarshal(data, &dico)
	if err != nil {
		return dico, err
	}

	return dico, nil
}

func (fv *FileVault) init() error {
	hash, err := readSecret()
	if err != nil {
		return err
	}
	fv.hash = hash

	dirVault, err := maybeCreateDir()
	if err != nil {
		return err
	}

	newPath := filepath.Join(dirVault, fv.Name)
	if _, err := os.Stat(newPath); os.IsNotExist(err) {
		// Migration: if an entry with this name exists at the legacy shared
		// path (~/.file-vault/), copy it into the per-app dir before creating
		// an empty file. Remove this fallback in a future release.
		if legacy, lerr := legacyVaultPath(fv.Name); lerr == nil {
			if data, rerr := ioutil.ReadFile(legacy); rerr == nil {
				return ioutil.WriteFile(newPath, data, 0600)
			}
		}
		if _, err := os.OpenFile(newPath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600); err != nil {
			return err
		}
	}

	return nil
}

func (fv *FileVault) decrypt(encrypted []byte) ([]byte, error) {
	cphr, err := aes.NewCipher(fv.hash)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(cphr)
	if err != nil {
		return nil, err
	}

	nonceSize := gcm.NonceSize()
	nonce, msg := encrypted[:nonceSize], encrypted[nonceSize:]
	data, err := gcm.Open(nil, nonce, msg, nil)
	if err != nil {
		return nil, err
	}

	return data, nil
}

func (fv *FileVault) encrypt(data []byte) ([]byte, error) {
	cphr, err := aes.NewCipher(fv.hash)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(cphr)
	if err != nil {
		return nil, err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}

	encrypted := gcm.Seal(nonce, nonce, data, nil)

	return encrypted, nil
}

// readSecret derives the AES key for the file vault from an explicit
// user-provided secret. Either <APPNAME>_VAULT_SECRET (a literal secret) or
// <APPNAME>_VAULT_SECRET_FILE (a path whose contents will be hashed) must be
// set; otherwise this returns an error rather than silently falling back to
// any on-disk key. The previous behaviour — implicitly hashing
// ~/.ssh/id_rsa — coupled vault recoverability to the user's SSH key
// without ever saying so, and broke on environments with no ~/.ssh (CI
// runners, freshly provisioned VMs).
func readSecret() ([]byte, error) {
	ctx, err := context.AppContext()
	if err != nil {
		return nil, err
	}

	if secret := os.Getenv(ctx.VaultSecretEnvVar()); secret != "" {
		hash := sha256.Sum256([]byte(secret))
		return hash[:], nil
	}

	if secretFile := os.Getenv(ctx.VaultSecretFileEnvVar()); secretFile != "" {
		data, err := ioutil.ReadFile(secretFile)
		if err != nil {
			return nil, fmt.Errorf("vault secret file %s: %w", secretFile, err)
		}
		hash := sha256.Sum256(data)
		return hash[:], nil
	}

	return nil, fmt.Errorf("vault secret not configured: set %s (literal) or %s (path to key material)",
		ctx.VaultSecretEnvVar(), ctx.VaultSecretFileEnvVar())
}

func maybeCreateDir() (string, error) {
	dirVault, err := appVaultDir()
	if err != nil {
		return "", err
	}

	if _, err := os.Stat(dirVault); os.IsNotExist(err) {
		if err := os.MkdirAll(dirVault, 0700); err != nil {
			return "", err
		}
	}

	return dirVault, nil
}

// appVaultDir returns <AppDir>/file-vault, scoping the vault under the
// per-launcher tree so two binaries with different names don't share state.
// AppDir resolution mirrors config.AppDir() (env override → ~/.appname) but
// is inlined here to avoid a circular import (config → helper → gvault).
func appVaultDir() (string, error) {
	ctx, err := context.AppContext()
	if err != nil {
		return "", err
	}

	base := os.Getenv(ctx.AppHomeEnvVar())
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		base = filepath.Join(home, ctx.AppDirname())
	}
	return filepath.Join(base, "file-vault"), nil
}

// legacyVaultPath returns the pre-migration location ~/.file-vault/<name>.
// Used only to read entries that were written before the per-app dir layout
// landed; new writes always go to appVaultDir(). Remove once migration window
// closes.
func legacyVaultPath(name string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".file-vault", name), nil
}
