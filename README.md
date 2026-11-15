# keyboardctl

纯软件键盘输入模拟器（Windows）——无需物理硬件。应用程序接收到的按键与真实键盘按下完全一致。

---

## 架构

```
用户态 (Ring 3)
└── Go 控制程序 (keyboardctl.exe)
    └── 业务逻辑、IOCTL 通信 → \\.\KbdFilter

内核态 (Ring 0)
└── C 键盘驱动 (keyboardsimulator.sys)
    ├── WDF 虚拟设备 + 符号链接
    ├── IOCTL 请求处理队列
    └── 附加到 \\Device\\KeyboardClass0 → 事件注入
```

```
┌─────────────────────────────────────────────────────┐
│          用户应用程序 (Ring 3)                      │
│     (记事本、浏览器、游戏, …)                       │
└─────────────────────┬───────────────────────────────┘
                      │  Win32 / Raw Input
                      ▼
┌─────────────────────────────────────────────────────┐
│          win32k.sys / RawInput 子系统               │
└─────────────────────┬───────────────────────────────┘
                      │  ClassService 回调
                      ▼
┌─────────────────────────────────────────────────────┐
│     kbdclass.sys — 键盘类驱动                       │
│     \\Device\\KeyboardClass0                        │
└─────────────────────┬───────────────────────────────┘
                      │  IOCTL_INTERNAL_KEYBOARD_CONNECT
                      ▼
┌─────────────────────────────────────────────────────┐
│     keyboardsimulator.sys (Ring 0, KMDF)            │
│     \\Device\\KbdFilter                             │
│     \\.\KbdFilter  (符号链接)                       │
└─────────────────────┬───────────────────────────────┘
                      │  IOCTL_KEYBOARD_INJECT (DeviceIoControl)
                      ▼
┌─────────────────────────────────────────────────────┐
│     keyboardctl.exe (Ring 3, Go)                    │
│     -key / -text / -combo / -hold / -release        │
└─────────────────────────────────────────────────────┘
```

### 内核驱动 (`driver/`)

| 文件 | 用途 |
|------|------|
| `driver.h` | 共享定义：`DEVICE_CONTEXT`、`IOCTL_KEYBOARD_INJECT`、函数原型 |
| `driver.c` | `DriverEntry`、`EvtDeviceAdd` — WDF 设备与符号链接创建 |
| `queue.c`  | `QueueInitialize`、`EvtIoDeviceControl` — IOCTL 分发 |
| `inject.c` | `ConnectKeyboardStack`、`InjectKeyboardEvents` — 附加到 `KeyboardClass0` 并调用其 `ClassService` 回调 |
| `sources`  | WDK (传统) 构建配置 |
| `makefile` | WDK 构建存根 |

### Go 用户态 (`cmd/keyboardctl/`)

| 文件 | 用途 |
|------|------|
| `scancodes.go` | `ScanCode` 类型、所有 PC/AT 扫描码、`ParseScanCode`、`RuneToScanCode` |
| `driver.go` | `KeyboardDriver` 结构体、`Open`/`Close`、`SendKey`、`sendBatch`、原始 IOCTL |
| `keyboard.go` | `PressKey`、`ReleaseKey`、`TapKey`、`TypeText`、`PressModifierAndKey` |
| `main.go` | CLI：`-key`、`-text`、`-combo`、`-hold`、`-release`、`-delay` |
| `scancodes_test.go` | 扫描码单元测试（无需驱动即可运行） |

---

## 核心数据结构

```c
// KEYBOARD_INPUT_DATA (ntddkbd.h) — 通过 IOCTL_KEYBOARD_INJECT 发送
typedef struct _KEYBOARD_INPUT_DATA {
    USHORT UnitId;           // 键盘单元标识符（首个键盘为 0）
    USHORT MakeCode;         // PC/AT 扫描码 (Set 1)
    USHORT Flags;            // KEY_MAKE=0, KEY_BREAK=1, KEY_E0=2, KEY_E1=4
    USHORT Reserved;         // 必须为 0
    ULONG  ExtraInformation; // 必须为 0
} KEYBOARD_INPUT_DATA;
```

```go
// Go 语言对应的 KEYBOARD_INPUT_DATA
type KeyboardInputData struct {
    UnitId           uint16
    MakeCode         uint16
    Flags            uint16
    Reserved         uint16
    ExtraInformation uint32
}
```

---

## IOCTL 协议

```
IOCTL_KEYBOARD_INJECT = CTL_CODE(FILE_DEVICE_KEYBOARD, 0x800,
                                  METHOD_BUFFERED, FILE_WRITE_ACCESS)
                      = 0x000BA000
```

将一个或多个 `KEYBOARD_INPUT_DATA` 结构体作为输入缓冲区写入。
扩展键（方向键、Insert、Delete、Home、End 等）必须设置 `Flags |= KEY_E0`。

---

## 常用扫描码

| 按键 | 扫描码 | 按键 | 扫描码 |
|------|--------|------|--------|
| A–Z | 0x1E, 0x30, 0x2E, … | 0–9 | 0x0B, 0x02–0x0A |
| Enter | 0x1C | Space | 0x39 |
| LShift | 0x2A | RShift | 0x36 |
| LCtrl | 0x1D | RCtrl | E0+0x1D |
| LAlt | 0x38 | RAlt | E0+0x38 |
| Up ↑ | E0+0x48 | Down ↓ | E0+0x50 |
| Left ← | E0+0x4B | Right → | E0+0x4D |
| F1–F10 | 0x3B–0x44 | F11 | 0x57 | F12 | 0x58 |

---

## 系统要求

| 组件 | 要求 |
|------|------|
| 操作系统 | Windows 10 / 11 x64 |
| 权限 | 管理员（驱动安装与运行） |
| Go | ≥ 1.23（仅构建 CLI 需要） |
| WDK | Windows Driver Kit（仅构建驱动需要） |

> **驱动签名**：开发阶段可用 `bcdedit /set testsigning on` 开启测试签名模式
> （`install.bat` 会自动完成）。生产部署需要 EV 代码签名证书。

---

## 构建

### 一键安装（推荐）

在 Windows 上双击 `install.bat` 即可。脚本会自动：

1. 请求管理员权限（UAC）
2. 按需开启测试签名模式并提示重启
3. 注册并启动内核驱动服务
4. 用 Go 构建 `keyboardctl.exe`（若已安装 Go）
5. 把项目目录加入用户 `PATH`

之后打开**新的**终端，直接运行 `keyboardctl -key a` 即可。

### 手动构建驱动（Windows，需 VS + WDK）

```bat
cd driver
msbuild driver.vcxproj /p:Configuration=Release /p:Platform=x64
```

输出：`driver\x64\Release\keyboardsimulator.sys`

> 旧版 WDK 的 `build -cZ` 工具已不再随当前 WDK 发布，请使用 MSBuild。

### 构建 Go CLI

```bat
make build          :: 输出 build\keyboardctl.exe
make test           :: 运行单元测试
make help           :: 查看所有可用目标
```

> 需要 Windows 上的 GNU make。也可以直接调用 Go：
> `go build -o build\keyboardctl.exe .\cmd\keyboardctl`

---

## 快速开始（Windows，管理员权限）

```bat
:: 1. 安装（双击，或管理员终端运行）
install.bat

:: 2. 按下 A
keyboardctl -key a

:: 3. 输入文本，按键间隔 100ms
keyboardctl -text "Hello, World!" -delay 100

:: 4. Ctrl+C
keyboardctl -combo ctrl c

:: 5. Alt+F4
keyboardctl -combo lalt f4

:: 6. 查看版本
keyboardctl -version

:: 7. 完成后卸载驱动
uninstall.bat
```

---

## CLI 参考

```
keyboardctl -key    <key>                   按下并释放一个键
keyboardctl -hold   <key>                   仅按下（key-down）
keyboardctl -release <key>                  仅释放（key-up）
keyboardctl -combo  <modifier> <key>        组合键（如 Ctrl+C）
keyboardctl -text   <string> [-delay <ms>]  逐字符输入字符串
keyboardctl -version                        打印版本并退出
```

按键参数：十六进制扫描码（`0x1E`）或不区分大小写的名称（`a`、`enter`、`ctrl`、`f12`、`up` 等）。

---

## Go 库用法

```go
drv, err := Open()   // 打开 \\.\KbdFilter
defer drv.Close()

drv.TapKey(ScancodeA)                             // 按下并释放 A（含真实按住时间）
drv.TypeText("Hello!", 50*time.Millisecond)       // 带延迟和抖动的输入
drv.PressModifierAndKey(ScancodeLCtrl, ScancodeC) // Ctrl+C
drv.PressKey(ScancodeLShift)                      // 按住 Shift
drv.ReleaseKey(ScancodeLShift)                    // 释放 Shift
```

---

## 测试

```bat
go test .\cmd\keyboardctl\
```

扫描码单元测试不依赖驱动，可在任意 Windows 环境运行。

---

## 文件结构

```
keyboardctl/
├── driver/                  # C 内核驱动 (Windows KMDF)
│   ├── driver.h             # 数据结构、IOCTL 码、函数原型
│   ├── driver.c             # DriverEntry, EvtDeviceAdd
│   ├── queue.c              # IOCTL 队列与处理器
│   ├── inject.c             # 键盘栈连接、事件注入
│   ├── driver.vcxproj       # MSBuild 工程（VS + WDK）
│   ├── sources              # WDK 传统构建配置（旧版 WDK）
│   └── makefile             # WDK 构建存根
├── cmd/
│   └── keyboardctl/         # Go CLI + 库
│       ├── main.go          # CLI 入口
│       ├── driver.go        # KeyboardDriver: Open/Close/SendKey/sendBatch
│       ├── keyboard.go      # PressKey/ReleaseKey/TapKey/TypeText/PressModifierAndKey
│       ├── scancodes.go     # 扫描码常量 + 字符映射
│       └── scancodes_test.go # 单元测试
├── install.bat              # 一键安装（提权 + 驱动 + 构建 + PATH）
├── uninstall.bat            # 驱动卸载
├── Makefile
├── go.mod
├── LICENSE                  # GPL-2.0
└── README.md
```

---

## 许可证

GPL-2.0，详见 [LICENSE](LICENSE)。
