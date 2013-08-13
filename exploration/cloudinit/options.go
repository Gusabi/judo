// Copyright 2011, 2012, 2013 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package cloudinit

import (
	"fmt"
	"strings"

	"launchpad.net/juju-core/utils"
)

// SetAttr sets an arbitrary attribute in the cloudinit config.
// If value is nil the attribute will be deleted; otherwise
// the value will be marshalled according to the rules
// of the goyaml Marshal function.
func (cfg *Config) SetAttr(name string, value interface{}) {
	cfg.set(name, value != nil, value)
}

// SetUser sets the user name that will be used for some other options.
// The user will be assumed to already exist in the machine image.
// The default user is "ubuntu".
func (cfg *Config) SetUser(user string) {
	cfg.set("user", user != "", user)
}

// SetAptUpgrade sets whether cloud-init runs "apt-get upgrade"
// on first boot.
func (cfg *Config) SetAptUpgrade(yes bool) {
	cfg.set("apt_upgrade", yes, yes)
}

// SetUpdate sets whether cloud-init runs "apt-get update"
// on first boot.
func (cfg *Config) SetAptUpdate(yes bool) {
	cfg.set("apt_update", yes, yes)
}

// SetAptProxy sets the URL to be used as the apt
// proxy.
func (cfg *Config) SetAptProxy(url string) {
	cfg.set("apt_proxy", url != "", url)
}

// SetAptMirror sets the URL to be used as the apt
// mirror site. If not set, the URL is selected based
// on cloud metadata in EC2 - <region>.archive.ubuntu.com
func (cfg *Config) SetAptMirror(url string) {
	cfg.set("apt_mirror", url != "", url)
}

// SetAptPreserveSourcesList sets whether /etc/apt/sources.list
// is overwritten by the mirror. If true, SetAptMirror above
// will have no effect.
func (cfg *Config) SetAptPreserveSourcesList(yes bool) {
	cfg.set("apt_mirror", yes, yes)
}

// AddAptSource adds an apt source. The key holds the
// public key of the source, in the form expected by apt-key(8).
func (cfg *Config) AddAptSource(name, key string) {
	src, _ := cfg.attrs["apt_sources"].([]*source)
	cfg.attrs["apt_sources"] = append(src,
		&source{
			Source: name,
			Key:    key,
		})
}

// AddAptSource adds an apt source. The public key for the
// source is retrieved by fetching the given keyId from the
// GPG key server at the given address.
func (cfg *Config) AddAptSourceWithKeyId(name, keyId, keyServer string) {
	src, _ := cfg.attrs["apt_sources"].([]*source)
	cfg.attrs["apt_sources"] = append(src,
		&source{
			Source:    name,
			KeyId:     keyId,
			KeyServer: keyServer,
		})
}

// SetDebconfSelections provides preseeded debconf answers
// for the boot process. The given answers will be used as input
// to debconf-set-selections(1).
func (cfg *Config) SetDebconfSelections(answers string) {
	cfg.set("debconf_selections", answers != "", answers)
}

// AddPackage adds a package to be installed on first boot.
// If any packages are specified, "apt-get update"
// will be called.
func (cfg *Config) AddPackage(name string) {
	pkgs, _ := cfg.attrs["packages"].([]string)
	cfg.attrs["packages"] = append(pkgs, name)
}

func (cfg *Config) addCmd(kind string, c *command) {
	cmds, _ := cfg.attrs[kind].([]*command)
	cfg.attrs[kind] = append(cmds, c)
}

// AddRunCmd adds a command to be executed
// at first boot. The command will be run
// by the shell with any metacharacters retaining
// their special meaning (that is, no quoting takes place).
func (cfg *Config) AddRunCmd(cmd string) {
	cfg.addCmd("runcmd", &command{literal: cmd})
}

// AddRunCmdArgs is like AddRunCmd except that the command
// will be executed with the given arguments properly quoted.
func (cfg *Config) AddRunCmdArgs(args ...string) {
	cfg.addCmd("runcmd", &command{args: args})
}

// AddBootCmd is like AddRunCmd except that the
// command will run very early in the boot process,
// and it will run on every boot, not just the first time.
func (cfg *Config) AddBootCmd(cmd string) {
	cfg.addCmd("bootcmd", &command{literal: cmd})
}

// AddBootCmdArgs is like AddBootCmd except that the command
// will be executed with the given arguments properly quoted.
func (cfg *Config) AddBootCmdArgs(args ...string) {
	cfg.addCmd("bootcmd", &command{args: args})
}

// SetDisableEC2Metadata sets whether access to the
// EC2 metadata service is disabled early in boot
// via a null route ( route del -host 169.254.169.254 reject).
func (cfg *Config) SetDisableEC2Metadata(yes bool) {
	cfg.set("disable_ec2_metadata", yes, yes)
}

// SetFinalMessage sets to message that will be written
// when the system has finished booting for the first time.
// By default, the message is:
// "cloud-init boot finished at $TIMESTAMP. Up $UPTIME seconds".
func (cfg *Config) SetFinalMessage(msg string) {
	cfg.set("final_message", msg != "", msg)
}

// SetLocale sets the locale; it defaults to en_US.UTF-8.
func (cfg *Config) SetLocale(locale string) {
	cfg.set("locale", locale != "", locale)
}

// AddMount adds a mount point. The given
// arguments will be used as a line in /etc/fstab.
func (cfg *Config) AddMount(args ...string) {
	mounts, _ := cfg.attrs["mounts"].([][]string)
	cfg.attrs["mounts"] = append(mounts, args)
}

// OutputKind represents a destination for command output.
type OutputKind string

const (
	OutInit   OutputKind = "init"
	OutConfig OutputKind = "config"
	OutFinal  OutputKind = "final"
	OutAll    OutputKind = "all"
)

// SetOutput specifies destination for command output.
// Valid values for the kind "init", "config", "final" and "all".
// Each of stdout and stderr can take one of the following forms:
//   >>file
//       appends to file
//   >file
//       overwrites file
//   |command
//       pipes to the given command.
func (cfg *Config) SetOutput(kind OutputKind, stdout, stderr string) {
	out, _ := cfg.attrs["output"].(map[string]interface{})
	if out == nil {
		out = make(map[string]interface{})
	}
	if stderr == "" {
		out[string(kind)] = stdout
	} else {
		out[string(kind)] = []string{stdout, stderr}
	}
	cfg.attrs["output"] = out
}

// AddSSHKey adds a pre-generated ssh key to the
// server keyring. Keys that are added like this will be
// written to /etc/ssh and new random keys will not
// be generated.
func (cfg *Config) AddSSHKey(keyType SSHKeyType, keyData string) {
	keys, _ := cfg.attrs["ssh_keys"].(map[SSHKeyType]string)
	if keys == nil {
		keys = make(map[SSHKeyType]string)
		cfg.attrs["ssh_keys"] = keys
	}
	keys[keyType] = keyData
}

// SetDisableRoot sets whether ssh login is disabled to the root account
// via the ssh authorized key associated with the instance metadata.
// It is true by default.
func (cfg *Config) SetDisableRoot(disable bool) {
	// note that disable_root defaults to true, so we include
	// the option only if disable is false.
	cfg.set("disable_root", !disable, disable)
}

// AddSSHAuthorizedKey adds a set of keys in
// ssh authorized_keys format (see ssh(8) for details)
// that will be added to ~/.ssh/authorized_keys for the
// configured user (see SetUser).
func (cfg *Config) AddSSHAuthorizedKeys(keys string) {
	akeys, _ := cfg.attrs["ssh_authorized_keys"].([]string)
	lines := strings.Split(keys, "\n")
	for _, line := range lines {
		if line == "" || line[0] == '#' {
			continue
		}
		akeys = append(akeys, line)
	}
	cfg.attrs["ssh_authorized_keys"] = akeys
}

// AddScripts is a simple shorthand for calling AddRunCmd multiple times.
func (cfg *Config) AddScripts(scripts ...string) {
	for _, s := range scripts {
		cfg.AddRunCmd(s)
	}
}

// AddFile will add multiple run_cmd entries to safely set the contents of a
// specific file to the requested contents.
func (cfg *Config) AddFile(filename, data string, mode uint) {
	p := shquote(filename)
	cfg.AddScripts(
		fmt.Sprintf("install -m %o /dev/null %s", mode, p),
		fmt.Sprintf("echo %s > %s", shquote(data), p),
	)
}

func shquote(p string) string {
	return utils.ShQuote(p)
}

// TODO
// byobu
// grub_dpkg
// mcollective
// phone_home
// puppet
// resizefs
// rightscale_userdata
// rsyslog
// scripts_per_boot
// scripts_per_instance
// scripts_per_once
// scripts_user
// set_hostname
// set_passwords
// ssh_import_id
// timezone
// update_etc_hosts
// update_hostname
