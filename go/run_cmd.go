package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"sshpocket-go/internal/cli"
	"sshpocket-go/internal/doctor"
	"sshpocket-go/internal/eject"
	"sshpocket-go/internal/report"
	"sshpocket-go/internal/runner"
	"sshpocket-go/internal/selfupdate"
	"sshpocket-go/internal/servers"
	"sshpocket-go/internal/settings"
)

// `sshpocket-go run [-servers D] [-components D] [-state D] <paket> <eylem> <sunucu>`: kapılar → yürütme → sonuç akışının
// Go uygulaması (internal/runner). Çıkış kodu kabuk motoruyla aynıdır: 0 başarı, eylemin kodu, 2 kullanım/geçersiz girdi,
// 190 çalıştırılmadı. Arayüz bunu varsayılan olarak pty altında çalıştırır (kabuğa dönüş: SSHPOCKET_SHELL_RUN=1).
//
// Parolalı anahtar ajanda değilse Keychain parolasıyla eklemeyi dener (runner.AgentAdder; gerçek Keychain ile henüz denenmedi).
func runCommand(args []string) int {
	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	servers := fs.String("servers", "", "sunucular klasörü")
	comps := fs.String("components", "", "components klasörü")
	state := fs.String("state", "", "durum/günlük klasörü")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() != 3 || *servers == "" || *comps == "" {
		fmt.Fprintln(os.Stderr, "kullanım: sshpocket-go run -servers <klasör> -components <klasör> [-state <klasör>] <paket> <eylem> <sunucu>")
		return 2
	}
	st := *state
	if st == "" {
		st = defaultStateDir()
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM, syscall.SIGHUP)
	defer stop()
	cli := &runner.CLI{
		ComponentsDir: *comps,
		LockPath:      filepath.Join(filepath.Dir(filepath.Clean(*comps)), "components.lock"),
		ServersDir:    *servers,
		StateDir:      st,
		Ctx:           ctx,
		Stdin:         os.Stdin,
		Stdout:        os.Stdout,
		Stderr:        os.Stderr,
	}
	return cli.Run(fs.Arg(0), fs.Arg(1), fs.Arg(2))
}

// goRunEnabled: arayüz eylem yürütmeyi Go yürütücüsüyle (internal/runner) yapsın mı. VARSAYILAN AÇIK (2026-11-07'den beri).
// Kabuk motoruna dönüş yolu: SSHPOCKET_SHELL_RUN=1 (ya da eski anahtar SSHPOCKET_GO_RUN=0). Kabuk motoru sistem/Main.sh ister.
func goRunEnabled() bool {
	return os.Getenv("SSHPOCKET_SHELL_RUN") != "1" && os.Getenv("SSHPOCKET_GO_RUN") != "0"
}

// subcommand: GUI'nin pty altında başlattığı etkileşimli alt komutlar. İstem metinleri kabuk motoruyla aynıdır.
//
//	sshpocket-go run …                         paket eylemi
//	sshpocket-go keycopy <kullanıcı> <ip> <port> <anahtar>   (ssh_copy_key)
//	sshpocket-go server-remove <servers-klasörü> <ad>        (ModuleManager.sh remove)
//	sshpocket-go components [-root D] list|sync|update|upgrade|check-new|prune|validate|report|run …   (--components)
//	sshpocket-go servers [-servers D] list | import-ssh-config [dosya] | knock <ad> | keychain <ad>
//	sshpocket-go settings [get <ANAHTAR> | set <ANAHTAR> <değer>]
//	sshpocket-go doctor | version | update [--check] | eject [USB-yolu]
//	sshpocket-go report -servers D -components D [<paket> [<eski-sürüm>]]   (comp_report_pkg / comp_report_all)
func subcommand(args []string) (int, bool) {
	if len(args) == 0 {
		return 0, false
	}
	con := servers.Console{In: bufio.NewReader(os.Stdin), Out: os.Stdout, Err: os.Stderr, ChildIn: os.Stdin}
	switch args[0] {
	case "run":
		return runCommand(args[1:]), true
	case "keycopy":
		if len(args) != 5 {
			fmt.Fprintln(os.Stderr, "kullanım: sshpocket-go keycopy <kullanıcı> <ip> <port> <anahtar>")
			return 2, true
		}
		if err := servers.ValidateTarget(args[2], args[1], args[3]); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 2, true
		}
		return servers.KeyCopy(con, args[1], args[2], args[3], args[4]), true
	case "components":
		return componentsCommand(args[1:]), true
	case "doctor":
		return doctor.Run(os.Stdout, doctor.Options{}), true
	case "version", "--version", "-V":
		fmt.Println("sshpocket " + versionOf(findRoot("")))
		return 0, true
	case "update", "--update":
		return updateCommand(args[1:]), true
	case "--check-update":
		return updateCommand([]string{"--check"}), true
	case "agent-add":
		// sshpocket-go agent-add <anahtar-yolu> <ÖNEK>: parolalı anahtarı Keychain parolasıyla ssh-agent'a ekler (yanlışsa yeniden kaydetmeyi sorar).
		if len(args) != 3 || !servers.NameValid(strings.ToLower(args[2])) {
			fmt.Fprintln(os.Stderr, "kullanım: sshpocket-go agent-add <anahtar-yolu> <ÖNEK>   (ÖNEK = sunucu adının büyük harfi)")
			return 2, true
		}
		rd := bufio.NewReader(os.Stdin)
		ad := &runner.AgentAdder{Err: os.Stderr, Ask: func(p string) (string, bool) {
			fmt.Fprint(os.Stderr, p)
			l, err := rd.ReadString('\n')
			if err != nil && l == "" {
				return "", false
			}
			return strings.TrimSuffix(l, "\n"), true
		}}
		if ad.AddRetry(args[2], args[1]) != nil {
			return 1, true
		}
		return 0, true
	case "servers":
		return serversCommand(args[1:]), true
	case "settings":
		return settingsCommand(args[1:]), true
	case "eject":
		usb := findRoot("")
		if len(args) > 1 {
			usb = args[1]
		}
		return eject.Run(os.Stdout, usb), true
	case "report":
		return reportCommand(args[1:]), true
	case "server-remove":
		if len(args) != 3 {
			fmt.Fprintln(os.Stderr, "kullanım: sshpocket-go server-remove <servers-klasörü> <ad>")
			return 2, true
		}
		return servers.RemoveInteractive(con, servers.Keychain{}, args[1], args[2]), true
	}
	return 0, false
}

// selfExe: bu ikilinin yolu (alt komutları pty altında yeniden çalıştırmak için).
func selfExe() string {
	if p, err := os.Executable(); err == nil {
		return p
	}
	return os.Args[0]
}

// reportCommand: paket sürüm farkı raporu (yalnızca okur).
func reportCommand(args []string) int {
	fs := flag.NewFlagSet("report", flag.ContinueOnError)
	srv := fs.String("servers", "", "sunucular klasörü")
	comps := fs.String("components", "", "components klasörü")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *srv == "" || *comps == "" || fs.NArg() > 2 {
		fmt.Fprintln(os.Stderr, "kullanım: sshpocket-go report -servers <klasör> -components <klasör> [<paket> [<eski-sürüm>]]")
		return 2
	}
	o := report.Options{ComponentsDir: *comps, ServersDir: *srv}
	if fs.NArg() == 0 {
		report.All(os.Stdout, os.Stderr, o)
		return 0
	}
	old := ""
	if fs.NArg() == 2 {
		old = fs.Arg(1)
	}
	return report.Pkg(os.Stdout, os.Stderr, o, fs.Arg(0), old)
}

// findRoot: kök klasör (içinde components/, components.lock, sistem/index_signers, VERSION). Sıra: -root bayrağı, SSHPOCKET_ROOT,
// ikilinin klasörü ya da bir üstü. (Yayın biçimi kararlaşınca burası güncellenir.)
func findRoot(flagRoot string) string {
	if flagRoot != "" {
		return flagRoot
	}
	if r := os.Getenv("SSHPOCKET_ROOT"); r != "" {
		return r
	}
	exe := filepath.Dir(selfExe())
	for _, d := range []string{exe, filepath.Dir(exe)} {
		if st, err := os.Stat(filepath.Join(d, "components.lock")); err == nil && !st.IsDir() {
			return d
		}
	}
	return filepath.Dir(exe)
}

// buildVersion: yayın derlemesinde -ldflags "-X main.buildVersion=X.Y.Z" ile gömülür (ikilinin kendi sürümü).
var buildVersion string

func versionOf(root string) string {
	if buildVersion != "" {
		return buildVersion
	}
	b, err := os.ReadFile(filepath.Join(root, "VERSION"))
	if err != nil {
		return "unknown"
	}
	v := strings.Join(strings.Fields(strings.SplitN(string(b), "\n", 2)[0]), "")
	if v == "" {
		return "unknown"
	}
	return v
}

// componentsCommand: `--components <alt komut>` (ComponentsCLI.sh components_cli).
func componentsCommand(args []string) int {
	fs := flag.NewFlagSet("components", flag.ContinueOnError)
	rootF := fs.String("root", "", "kök klasör (components/, components.lock, VERSION burada)")
	srvF := fs.String("servers", "", "sunucular klasörü (varsayılan: <kök>/servers)")
	compF := fs.String("components", "", "components klasörü (varsayılan: <kök>/components)")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	root := findRoot(*rootF)
	srv, comp := *srvF, *compF
	if srv == "" {
		srv = os.Getenv("SSHPOCKET_SERVERS_DIR")
	}
	if srv == "" {
		srv = filepath.Join(root, "servers")
	}
	if comp == "" {
		comp = os.Getenv("SSHPOCKET_COMPONENTS_DIR")
	}
	if comp == "" {
		comp = filepath.Join(root, "components")
	}
	signers := os.Getenv("SSHPOCKET_INDEX_SIGNERS")
	if signers == "" {
		signers = signersPath(root, "index_signers")
	}
	c := &cli.Components{
		Dir: comp, LockPath: filepath.Join(filepath.Dir(filepath.Clean(comp)), "components.lock"), ServersDir: srv, SignersFile: signers,
		IndexBase: os.Getenv("SSHPOCKET_INDEX_URL"), SSHPocketVer: versionOf(root), In: os.Stdin, Out: os.Stdout, Err: os.Stderr,
	}
	sub, rest := "list", fs.Args()
	if len(rest) > 0 {
		sub, rest = rest[0], rest[1:]
	}
	switch sub {
	case "list":
		return c.List()
	case "sync":
		return c.Sync(len(rest) > 0 && rest[0] == "--yes")
	case "update":
		return c.Update()
	case "upgrade":
		return c.Upgrade(rest)
	case "check-new":
		return c.CheckNew()
	case "prune":
		return c.Prune(rest)
	case "validate":
		return c.Validate(rest)
	case "lock": // bakımcı aracı: scripts/lock-component.sh karşılığı
		rs := os.Getenv("LOCK_RELEASE_SIGNERS")
		if rs == "" {
			rs = signersPath(root, "release_signers")
		}
		return c.LockCommand(rest, os.Getenv("LOCK_COMPONENT_ARCHIVE"), os.Getenv("LOCK_COMPONENT_SIG"), rs)
	case "report":
		return reportCommand(append([]string{"-servers", srv, "-components", comp}, rest...))
	case "run":
		return runCommand(append([]string{"-servers", srv, "-components", comp}, rest...))
	}
	fmt.Fprintln(os.Stderr, "kullanım: components list | sync [--yes] | update | upgrade [--yes] [paket…] | check-new | prune [--yes] [--dry-run] | report [paket [eski-sürüm]] | run <paket> <eylem> <sunucu> | validate <paket|dizin|arşiv> [--strict] | lock <paket> <sürüm> [--repo sahip/depo] [--signers satır] [--write]")
	return 2
}

// updateCommand: imzalı kendini güncelleme (selfupdate). Başarılı olursa YENİ ikiliyle paketler kilide göre eşitlenir
// (kabuktaki `--update` sonrası `--components sync` ile aynı).
func updateCommand(args []string) int {
	check := len(args) > 0 && (args[0] == "--check" || args[0] == "--check-only")
	root := findRoot("")
	exe, err := filepath.EvalSymlinks(selfExe())
	if err != nil {
		exe = selfExe()
	}
	signers := signersPath(root, "release_signers")
	u := &selfupdate.Updater{SignersFile: signers, ExePath: exe, Version: versionOf(root), In: os.Stdin, Out: os.Stdout, Err: os.Stderr, Head: cli.HTTPHead}
	code, updated := u.Run(check)
	if code == 0 && updated {
		cmd := exec.Command(exe, "components", "-root", root, "sync")
		cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
		if err := cmd.Run(); err != nil {
			return 1
		}
	}
	return code
}

// signersPath: imza anahtarı dosyası: <kök>/<ad>; yoksa eski düzen <kök>/sistem/<ad>.
func signersPath(root, name string) string {
	p := filepath.Join(root, name)
	if _, err := os.Stat(p); err != nil {
		if q := filepath.Join(root, "sistem", name); fileExists(q) {
			return q
		}
	}
	return p
}

func fileExists(p string) bool {
	st, err := os.Stat(p)
	return err == nil && st.Mode().IsRegular()
}

// serversDir: -servers bayrağı, SSHPOCKET_SERVERS_DIR ya da <kök>/servers.
func serversDirOf(flagVal, root string) string {
	if flagVal != "" {
		return flagVal
	}
	if v := os.Getenv("SSHPOCKET_SERVERS_DIR"); v != "" {
		return v
	}
	return filepath.Join(root, "servers")
}

// serversCommand: sunucu yönetimi (ImportSSHConfig.sh, ModuleManager.sh list, Knock.sh knock_manage, ssh_keychain_manage).
func serversCommand(args []string) int {
	fs := flag.NewFlagSet("servers", flag.ContinueOnError)
	rootF := fs.String("root", "", "kök klasör")
	srvF := fs.String("servers", "", "sunucular klasörü (varsayılan: <kök>/servers)")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	root := findRoot(*rootF)
	dir := serversDirOf(*srvF, root)
	rest := fs.Args()
	usage := func() int {
		fmt.Fprintln(os.Stderr, "kullanım: servers [-servers D] list | import-ssh-config [dosya] | knock <ad> | knock-send <ad> | keychain <ad>")
		return 2
	}
	if len(rest) == 0 {
		return usage()
	}
	s := &cli.Servers{Dir: dir, In: bufio.NewReader(os.Stdin), Out: os.Stdout, Err: os.Stderr, ChildIn: os.Stdin, DefaultUser: os.Getenv("USER"), Askpass: selfExe()}
	switch rest[0] {
	case "list":
		names := servers.Names(dir)
		if len(names) == 0 {
			fmt.Println("  (sunucu yok)")
			return 0
		}
		for _, n := range names {
			fmt.Println(servers.ListLine(dir, n))
		}
		return 0
	case "import-ssh-config":
		cfg := filepath.Join(os.Getenv("HOME"), ".ssh", "config")
		if len(rest) > 1 {
			cfg = rest[1]
		}
		return s.ImportSSHConfig(cfg)
	case "knock":
		if len(rest) != 2 {
			return usage()
		}
		return s.KnockManage(rest[1])
	case "keychain":
		if len(rest) != 2 {
			return usage()
		}
		return s.KeychainManage(rest[1])
	case "knock-send":
		// ayarlı knock dizisini şimdi gönderir (bağlanmadan önce otomatik gönderilenin aynısı); etkin değilse bir şey yapmaz
		if len(rest) != 2 {
			return usage()
		}
		if err := cli.SendKnock(context.Background(), dir, rest[1]); err != nil {
			fmt.Fprintln(os.Stderr, "⚠️ ", err)
			return 1
		}
		fmt.Println("🚪 Knock gönderildi (ya da bu sunucuda etkin değil)")
		return 0
	}
	return usage()
}

// confFile: sshpocket.conf yolu (SSHPOCKET_CONF ya da <kök>/sshpocket.conf).
func confFile(root string) string {
	if v := os.Getenv("SSHPOCKET_CONF"); v != "" {
		return v
	}
	return filepath.Join(root, "sshpocket.conf")
}

// settingsCommand: sshpocket.conf okuma/yazma (Settings.sh). Argümansız: bilinen ayarların değeri ve kaynağı.
func settingsCommand(args []string) int {
	st := settings.Store{File: confFile(findRoot(""))}
	known := []string{"STATUS", "STATUS_TIMEOUT"}
	switch {
	case len(args) == 0:
		for _, k := range known {
			v, _ := st.Get(k)
			fmt.Printf("%-16s %-8s (%s)\n", k, v, st.Source(k))
		}
		return 0
	case args[0] == "get" && len(args) == 2:
		if v, ok := st.Get(args[1]); ok {
			fmt.Println(v)
			return 0
		}
		fmt.Fprintf(os.Stderr, "Bilinmeyen ayar: %s\n", args[1])
		return 1
	case args[0] == "set" && len(args) == 3:
		if err := st.Set(args[1], args[2]); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		return 0
	}
	fmt.Fprintln(os.Stderr, "kullanım: settings [get <ANAHTAR> | set <ANAHTAR> <değer>]   (ANAHTAR: STATUS | STATUS_TIMEOUT)")
	return 2
}
