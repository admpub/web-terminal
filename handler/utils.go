package handler

import (
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/admpub/web-terminal/config"
	"golang.org/x/crypto/ssh"
	"golang.org/x/text/transform"

	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/japanese"
	"golang.org/x/text/encoding/korean"
	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/encoding/traditionalchinese"
	"golang.org/x/text/encoding/unicode"
)

var (
	supportedCiphers = SupportedCiphers()
	commands         = map[string]string{}
)

func init() {
	fillCommands(config.ExecutableFolder)
}

func CharsetEncoding(charset string) encoding.Encoding {
	switch strings.ToUpper(charset) {
	case "GB18030":
		return simplifiedchinese.GB18030
	case "GB2312", "HZ-GB2312":
		return simplifiedchinese.HZGB2312
	case "GBK":
		return simplifiedchinese.GBK
	case "BIG5":
		return traditionalchinese.Big5
	case "EUC-JP":
		return japanese.EUCJP
	case "ISO2022JP":
		return japanese.ISO2022JP
	case "SHIFTJIS":
		return japanese.ShiftJIS
	case "EUC-KR":
		return korean.EUCKR
	case "UTF8", "UTF-8":
		return encoding.Nop
	case "UTF16-BOM", "UTF-16-BOM":
		return unicode.UTF16(unicode.BigEndian, unicode.UseBOM)
	case "UTF16-BE-BOM", "UTF-16-BE-BOM":
		return unicode.UTF16(unicode.BigEndian, unicode.UseBOM)
	case "UTF16-LE-BOM", "UTF-16-LE-BOM":
		return unicode.UTF16(unicode.LittleEndian, unicode.UseBOM)
	case "UTF16", "UTF-16":
		return unicode.UTF16(unicode.BigEndian, unicode.IgnoreBOM)
	case "UTF16-BE", "UTF-16-BE":
		return unicode.UTF16(unicode.BigEndian, unicode.IgnoreBOM)
	case "UTF16-LE", "UTF-16-LE":
		return unicode.UTF16(unicode.LittleEndian, unicode.IgnoreBOM)
	//case "UTF32", "UTF-32": return simplifiedchinese.GBK
	default:
		return nil
	}
}

func SupportedCiphers() []string {
	config := &ssh.ClientConfig{}
	config.SetDefaults()
	for _, cipher := range []string{"aes128-cbc"} {
		found := false
		for _, defaultCipher := range config.Ciphers {
			if cipher == defaultCipher {
				found = true
				break
			}
		}

		if !found {
			config.Ciphers = append(config.Ciphers, cipher)
		}
	}
	return config.Ciphers
}

func warp(dst io.ReadCloser, dump io.Writer) io.ReadCloser {
	if nil == dump {
		return dst
	}
	return &consoleReader{out: dump, dst: dst}
}

func decodeBy(charset string, dst io.Writer) io.Writer {
	switch strings.ToUpper(charset) {
	case "UTF-8", "UTF8":
		return dst
	}
	cs := CharsetEncoding(charset)
	if nil == cs {
		panic("charset '" + charset + "' is not exists.")
	}

	return transform.NewWriter(dst, cs.NewDecoder())
}

func matchBy(dst io.Writer, excepted string, cb func()) io.Writer {
	return &matchWriter{
		out:      dst,
		excepted: []byte(excepted),
		cb:       cb,
	}
}

func toInt(s string, v int) int {
	if value, e := strconv.ParseInt(s, 10, 0); nil == e {
		return int(value)
	}
	return v
}

func logString(ws io.Writer, msg string) {
	if nil != ws {
		io.WriteString(ws, "%tpt%"+msg)
	}
	log.Println(msg)
}

func fillCommands(executableFolder string) {
	for _, nm := range []string{"snmpget", "snmpgetnext", "snmpdf", "snmpbulkget",
		"snmpbulkwalk", "snmpdelta", "snmpnetstat", "snmpset", "snmpstatus",
		"snmptable", "snmptest", "snmptools", "snmptranslate", "snmptrap", "snmpusm",
		"snmpvacm", "snmpwalk", "wshell"} {
		if pa, ok := lookPath(executableFolder, nm); ok {
			commands[nm] = pa
		} else if pa, ok := lookPath(executableFolder, "netsnmp/"+nm); ok {
			commands[nm] = pa
		} else if pa, ok := lookPath(executableFolder, "net-snmp/"+nm); ok {
			commands[nm] = pa
		}
	}

	if pa, ok := lookPath(executableFolder, "tpt"); ok {
		commands["tpt"] = pa
	}
	if pa, ok := lookPath(executableFolder, "nmap/nping"); ok {
		commands["nping"] = pa
	}
	if pa, ok := lookPath(executableFolder, "nmap/nmap"); ok {
		commands["nmap"] = pa
	}
	if pa, ok := lookPath(executableFolder, "putty/plink", "ssh"); ok {
		commands["plink"] = pa
		commands["ssh"] = pa
	}
	if pa, ok := lookPath(executableFolder, "dig/dig", "dig"); ok {
		commands["dig"] = pa
	}
}

func saveSessionKey(pa string, args []string, wd string) {
	args = removeBatchOption(args)
	var cmd = exec.Command(pa, args...)
	if "" != wd {
		cmd.Dir = wd
	}

	timer := time.AfterFunc(1*time.Minute, func() {
		defer recover()
		cmd.Process.Kill()
	})
	cmd.Stdin = strings.NewReader("y\ny\ny\ny\ny\ny\ny\ny\n")
	cmd.Run()
	timer.Stop()
}

func lookPath(executableFolder string, alias ...string) (string, bool) {
	var names []string
	for _, aliasName := range alias {
		if runtime.GOOS == "windows" {
			names = append(names, aliasName, aliasName+".bat", aliasName+".com", aliasName+".exe")
		} else {
			names = append(names, aliasName, aliasName+".sh")
		}
	}

	for _, nm := range names {
		files := []string{nm,
			filepath.Join("bin", nm),
			filepath.Join("tools", nm),
			filepath.Join("runtime_env", nm),
			filepath.Join("..", nm),
			filepath.Join("..", "bin", nm),
			filepath.Join("..", "tools", nm),
			filepath.Join("..", "runtime_env", nm),
			filepath.Join(executableFolder, nm),
			filepath.Join(executableFolder, "bin", nm),
			filepath.Join(executableFolder, "tools", nm),
			filepath.Join(executableFolder, "runtime_env", nm),
			filepath.Join(executableFolder, "..", nm),
			filepath.Join(executableFolder, "..", "bin", nm),
			filepath.Join(executableFolder, "..", "tools", nm),
			filepath.Join(executableFolder, "..", "runtime_env", nm)}
		for _, file := range files {
			// fmt.Println("====", file)
			file = config.AbsPath(file)
			if st, e := os.Stat(file); nil == e && nil != st && !st.IsDir() {
				//fmt.Println("1=====", file, e)
				return file, true
			}
		}
	}

	for _, nm := range names {
		_, err := exec.LookPath(nm)
		if nil == err {
			return nm, true
		}
	}
	return "", false
}

func removeBatchOption(args []string) []string {
	offset := 0
	for idx, s := range args {
		if strings.ToLower(s) == "-batch" {
			continue
		}
		if offset != idx {
			args[offset] = s
		}
		offset++
	}
	return args[:offset]
}

func addMibDir(args []string) []string {
	hasMIBSDir := false
	for _, argument := range args {
		if "-M" == argument {
			hasMIBSDir = true
		}
	}

	if !hasMIBSDir {
		newArgs := make([]string, len(args)+2)
		newArgs[0] = "-M"
		newArgs[1] = config.Default.MIBSDir
		copy(newArgs[2:], args)
		args = newArgs
	}
	return args
}
