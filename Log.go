package hlog

import (
        "errors"
        "fmt"
        "os"
        "path"
        "runtime"
        "sync"
        "time"
)

type AtmLogSt struct {
        logName     string
        logLevel    uint
        logLevelStr [4]string
        funcNameFlg uint
        isShortFile bool
        rwLock      *sync.RWMutex
}

const (
        // 日志打印级别
        OFF_LEVEL   uint    = 0x0000
        ERR_LEVEL   uint    = 0x1000
        WARN_LEVEL  uint    = 0x2000
        DEBUG_LEVEL uint    = 0x3000
        INFO_LEVEL  uint    = 0x4000
)
const (
        // 日志输出位置<标准输出、文件>
        RPT_TO_FILE  uint    = 0X10
        RPT_TO_STOUT uint    = 0X20
)

// 全局日志级别
var g_logLevel uint = 0x0

const (
        // 全路径名
        FULL_FUNC = 0x10
        // 带有包名
        PACKAGE_FUNC = 0x20
        // 只有函数名
        SHORT_FUNC = 0x30
)

// 日志文件最大字节数 -> 10M
const MAX_LOG_FILE_SIZE = 1024 * 1024 * 10

// 字节拷贝
func byteCopy(dest, src []byte, iLen int) {
        for i := 0; i < iLen; i++ {
                dest[i] = src[i]
        }

}

// 初始化日志的打印级别
func InitLogLevel(debugLevel uint) {
        g_logLevel = debugLevel
}

// 初始化日志相关参数
func InitLog(logName string) *AtmLogSt {
        als := new(AtmLogSt)
        als.logName = logName
        als.logLevelStr[ERR_LEVEL>>12-1] = "ERR"
        als.logLevelStr[INFO_LEVEL>>12-1] = "INFO"
        als.logLevelStr[WARN_LEVEL>>12-1] = "WARN"
        als.logLevelStr[DEBUG_LEVEL>>12-1] = "DEBUG"
        als.funcNameFlg = FULL_FUNC
        als.isShortFile = false
        als.rwLock = new(sync.RWMutex)
        //als.logLevel = g_logLevel

        return als
}

func getFuncName(f string) string {
        iLen := len(f)
        v := make([]byte, iLen)

        iLen -= 1
        i := 0
        for iLen >= 0 {
                if f[iLen] == '/' {
                        iLen++
                        break
                }
                iLen--
                i++
        }
        byteCopy(v, []byte(f)[iLen:], i)

        return string(v[:i])
}

func getShortFuncName(f string) string {
        iLen := len(f)
        v := make([]byte, iLen)

        iLen -= 1
        i := 0
        for iLen >= 0 {
                if f[iLen] == '.' {
                        iLen++
                        break
                }
                iLen--
                i++
        }
        byteCopy(v, []byte(f)[iLen:], i)

        return string(v[:i])
}

// 更改打印日志级别
func chgLogLevel(debugSwitch uint) {
        g_logLevel = debugSwitch
}

// 更改文件名打印格式<长文件名[true]|短文件名[false]>
func (this *AtmLogSt) ChgLogFileStyle(v bool) {
        this.isShortFile = v
}

// 更改函数名打印格式<全路径[FULL_FUNC]|包函数名[PACKAGE_FUNC]|函数名[SHORT_FUNC]>
func (this *AtmLogSt) ChgLogFuncStyle(v uint) {
        this.funcNameFlg = v
}

// 将相关信息写入日志文件
func (als *AtmLogSt) WriteLog(LogLevel uint, LogMsg string, LogOutObj uint,
        DumpMsg []byte, DumpLen int) error {
        var pc uintptr
        var err error
        var LFile *os.File
        var WStr string

        if LogLevel > g_logLevel {
                return nil
        }

        switch LogLevel {
        case ERR_LEVEL:
                fallthrough
        case WARN_LEVEL:
                fallthrough
        case DEBUG_LEVEL:
                fallthrough
        case INFO_LEVEL:
        default:
                return errors.New("不可识别的日志级别!")
        }
        Ltime := time.Now()

        WStr = WStr + fmt.Sprintf("Pid [%d] | Time [%s] | Message: \n",
                os.Getpid(), Ltime.Format(time.ANSIC))

        // 获取上层函数的信息
        pc, FileName, LineNo, _ := runtime.Caller(1)
        f := runtime.FuncForPC(pc)
        FuncName := f.Name()

        if als.isShortFile {
                FileName = path.Base(FileName)
        }
        switch als.funcNameFlg {
        case PACKAGE_FUNC:
                FuncName = getFuncName(FuncName)
        case SHORT_FUNC:
                FuncName = getShortFuncName(FuncName)
        }

        WStr = WStr + fmt.Sprintf("%s>> File[%s] Function[%s] Line[%d] %s |\n",
                als.logLevelStr[LogLevel>>12-1], FileName, FuncName, LineNo, LogMsg)
        if RPT_TO_FILE^LogOutObj == 0 {

                LFile, err = os.OpenFile(als.logName, os.O_RDWR|os.O_CREATE|os.O_APPEND|os.O_EXCL, 0644)
                if err != nil {
                        if os.IsExist(err) {
                                LFile, err = os.OpenFile(als.logName, os.O_APPEND|os.O_RDWR, 0644)
                                if err != nil {
                                        return err
                                }
                        } else {
                                return err
                        }
                }
                defer LFile.Close()
                if fi, err := LFile.Stat(); err != nil {
                        return err
                } else {
                        if fi.Size()+int64(len(LogMsg)+DumpLen) > MAX_LOG_FILE_SIZE {
                                t := time.Now()
                                newName := fmt.Sprintf("%s.hwch.%04d%02d%02d%02d%02d%02d",
                                        als.logName, t.Year(), t.Month(), t.Day(), t.Hour(),
                                        t.Minute(), t.Second())
                                os.Rename(als.logName, newName)
                        }
                }
        } else {
                LFile = os.Stdout
        }

        if DumpMsg != nil && DumpLen > 0 {
                tStr := als.dumpMsgToString(DumpMsg, DumpLen)
                WStr = WStr + tStr
        }

        als.rwLock.Lock()
        LFile.WriteString(WStr)
        als.rwLock.Unlock()

        return nil
}

// 以ASCII码形式对照打印原始字符串
func (als *AtmLogSt) dumpMsgToString(DumpMsg []byte, DumpLen int) string {
        Count := 0
        Pos := 0

        WStr := "Displacement    ++00++01++02++03++04++05++06++07++08++09++10++11++12++13++" +
                "14++15++  ++ASCII  Value++\n"
        Len := len(WStr)

L:
        for DumpLen != 0 {
                WStr = WStr + fmt.Sprintf("%05d(%05X)      ",
                        Count, Count)
                Len += 18 /* length of strlen("%05d(%05x)     ++"); */
                if DumpLen < 16 {
                        TmpBuf := make([]byte, DumpLen)
                        copy(TmpBuf, DumpMsg[Pos:Pos+DumpLen])
                        i := 0
                        for i < DumpLen {
                                WStr = WStr + fmt.Sprintf("%02X  ", TmpBuf[i])
                                Len += 4
                                i++
                        }
                        for i < 16 {
                                WStr = WStr + "    "
                                Len += 4
                                i++
                        }
                        WStr = WStr + "  "
                        Len += 2
                        toPrintCh(TmpBuf, DumpLen)
                        WStr = WStr + string(TmpBuf) + "\n"
                        Len += DumpLen + 1
                        break L
                } else {
                        TmpBuf := make([]byte, 16)
                        copy(TmpBuf, DumpMsg[Pos:Pos+16])
                        DumpLen -= 16
                        WStr = WStr + fmt.Sprintf("%02X  %02X  %02X  %02X  "+
                                "%02X  %02X  %02X  %02X  "+
                                "%02X  %02X  %02X  %02X  "+
                                "%02X  %02X  %02X  %02X    ",
                                TmpBuf[0], TmpBuf[1], TmpBuf[2], TmpBuf[3],
                                TmpBuf[4], TmpBuf[5], TmpBuf[6], TmpBuf[7],
                                TmpBuf[8], TmpBuf[9], TmpBuf[10], TmpBuf[11],
                                TmpBuf[12], TmpBuf[13], TmpBuf[14], TmpBuf[15])
                        Len += 4*16 + 2
                        toPrintCh(TmpBuf, 16)
                        WStr = WStr + string(TmpBuf) + "\n"
                        Len += 17
                        Pos += 16
                }
                Count += 16
        }

        return WStr
}

// 将数字0转换为可见字符'*'
func toPrintCh(v []byte, slen int) {

        for i := 0; i < slen; i++ {
                if v[i] < 0x20 {
                        v[i] = '.'
                }
        }
}
