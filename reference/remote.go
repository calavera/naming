package reference

import (
	"errors"
	"fmt"
	"strings"

	"github.com/docker/naming/digest"
)

const (
	// DefaultTag defines the default tag used when performing images related actions and no tag or digest is specified
	DefaultTag = "latest"
	// DefaultHostname is the default built-in hostname
	DefaultHostname = "docker.io"
	// LegacyDefaultHostname is automatically converted to DefaultHostname
	LegacyDefaultHostname = "index.docker.io"
	// DefaultRepoPrefix is the prefix used for default repositories in default host
	DefaultRepoPrefix = "library/"
)

// RemoteNamed is an object with a full name
type RemoteNamed interface {
	Named
	// FullName returns full repository name with hostname, like "docker.io/library/ubuntu"
	FullName() string
	// Hostname returns hostname for the reference, like "docker.io"
	Hostname() string
	// RemoteName returns the repository component of the full name, like "library/ubuntu"
	RemoteName() string
}

// RemoteTagged is an object including a name and tag.
type RemoteTagged interface {
	RemoteNamed
	Tag() string
}

// RemoteCanonical reference is an object with a fully unique
// name including a name with hostname and digest
type RemoteCanonical interface {
	RemoteNamed
	Digest() digest.Digest
}

// ParseRemoteNamed parses s and returns a syntactically valid reference implementing
// the RemoteNamed interface. The reference must have a name, otherwise an error is
// returned.
// If an error was encountered it is returned, along with a nil Reference.
func ParseRemoteNamed(s string) (RemoteNamed, error) {
	named, err := ParseNamed(s)
	if err != nil {
		return nil, fmt.Errorf("Error parsing reference: %q is not a valid repository/tag", s)
	}
	r, err := WithRemoteName(named.Name())
	if err != nil {
		return nil, err
	}
	if canonical, isCanonical := named.(Canonical); isCanonical {
		return WithRemoteDigest(r, canonical.Digest())
	}
	if tagged, isTagged := named.(NamedTagged); isTagged {
		return WithRemoteTag(r, tagged.Tag())
	}
	return r, nil
}

// WithRemoteName returns a named object representing the given string. If the input
// is invalid ErrReferenceInvalidFormat will be returned.
func WithRemoteName(name string) (RemoteNamed, error) {
	name, err := normalize(name)
	if err != nil {
		return nil, err
	}
	if err := validateName(name); err != nil {
		return nil, err
	}
	r, err := WithName(name)
	if err != nil {
		return nil, err
	}
	return &remoteNamedRef{r}, nil
}

// WithRemoteTag combines the name from "name" and the tag from "tag" to form a
// reference incorporating both the name and the tag.
func WithRemoteTag(name Named, tag string) (RemoteTagged, error) {
	r, err := WithTag(name, tag)
	if err != nil {
		return nil, err
	}
	return &remoteTaggedRef{remoteNamedRef{r}}, nil
}

// WithRemoteDigest combines the name from "name" and the digest from "digest" to form
// a reference incorporating both the name and the digest.
func WithRemoteDigest(name Named, digest digest.Digest) (RemoteCanonical, error) {
	r, err := WithDigest(name, digest)
	if err != nil {
		return nil, err
	}
	return &remoteCanonicalRef{remoteNamedRef{r}}, nil
}

type remoteNamedRef struct {
	Named
}
type remoteTaggedRef struct {
	remoteNamedRef
}
type remoteCanonicalRef struct {
	remoteNamedRef
}

func (r *remoteNamedRef) FullName() string {
	hostname, remoteName := splitHostname(r.Name())
	return hostname + "/" + remoteName
}
func (r *remoteNamedRef) Hostname() string {
	hostname, _ := splitHostname(r.Name())
	return hostname
}
func (r *remoteNamedRef) RemoteName() string {
	_, remoteName := splitHostname(r.Name())
	return remoteName
}
func (r *remoteTaggedRef) Tag() string {
	return r.remoteNamedRef.Named.(NamedTagged).Tag()
}
func (r *remoteCanonicalRef) Digest() digest.Digest {
	return r.remoteNamedRef.Named.(Canonical).Digest()
}

// WithDefaultRemoteTag adds a default tag to a reference if it only has a repo name.
func WithDefaultRemoteTag(ref RemoteNamed) RemoteNamed {
	if IsRemoteNameOnly(ref) {
		ref, _ = WithRemoteTag(ref, DefaultTag)
	}
	return ref
}

// IsRemoteNameOnly returns true if reference only contains a repo name.
func IsRemoteNameOnly(ref Named) bool {
	if _, ok := ref.(RemoteTagged); ok {
		return false
	}
	if _, ok := ref.(RemoteCanonical); ok {
		return false
	}
	return true
}

// ParseIDOrReference parses string for a image ID or a reference. ID can be
// without a default prefix.
func ParseIDOrReference(idOrRef string) (digest.Digest, Named, error) {
	if err := digest.ValidateHex(idOrRef); err == nil {
		idOrRef = "sha256:" + idOrRef
	}
	if dgst, err := digest.ParseDigest(idOrRef); err == nil {
		return dgst, nil, nil
	}
	ref, err := ParseRemoteNamed(idOrRef)
	return "", ref, err
}

// splitHostname splits a repository name to hostname and remotename string.
// If no valid hostname is found, the default hostname is used. Repository name
// needs to be already validated before.
func splitHostname(name string) (hostname, remoteName string) {
	i := strings.IndexRune(name, '/')
	if i == -1 || (!strings.ContainsAny(name[:i], ".:") && name[:i] != "localhost") {
		hostname, remoteName = DefaultHostname, name
	} else {
		hostname, remoteName = name[:i], name[i+1:]
	}
	if hostname == LegacyDefaultHostname {
		hostname = DefaultHostname
	}
	if hostname == DefaultHostname && !strings.ContainsRune(remoteName, '/') {
		remoteName = DefaultRepoPrefix + remoteName
	}
	return
}

// normalize returns a repository name in its normalized form, meaning it
// will not contain default hostname nor library/ prefix for official images.
func normalize(name string) (string, error) {
	host, remoteName := splitHostname(name)
	if strings.ToLower(remoteName) != remoteName {
		return "", errors.New("invalid reference format: repository name must be lowercase")
	}
	if host == DefaultHostname {
		if strings.HasPrefix(remoteName, DefaultRepoPrefix) {
			return strings.TrimPrefix(remoteName, DefaultRepoPrefix), nil
		}
		return remoteName, nil
	}
	return name, nil
}

func validateName(name string) error {
	if err := digest.ValidateHex(name); err == nil {
		return fmt.Errorf("Invalid repository name (%s), cannot specify 64-byte hexadecimal strings", name)
	}
	return nil
}
