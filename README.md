# keyboardctl

纯软件键盘输入模拟器（Windows）——无需物理硬件。应用程序接收到的按键与真实键盘按下完全一致。

---

## 下载即用（推荐）

**不需要编译器，也不需要 WDK。** 到
[Releases](https://github.com/axetroy/keyboardctl/releases) 下载
`keyboardctl-<版本>-windows-x64.zip`，解压到任意目录，然后：

1. 双击 `install.bat`。它会请求管理员权限，自动完成：信任随包的测试证书、
   开启测试签名、通过 INF（`pnputil`）安装键盘过滤驱动并把它登记到键盘类
   `UpperFilters`、把当前目录加入用户 `PATH`。
2. 安装程序会询问**是否立即重启**。过滤驱动只在开机枚举键盘时才会挂进键盘栈，
   所以必须重启一次才会生效（开启测试签名同样需要重启）。选择 `Y` 会在 15 秒后
   自动重启；选择 `N` 请稍后自行重启。
3. 重启后打开一个**新的**终端，运行：

   ```bat
   keyboardctl -selftest
   keyboardctl -key a
   ```

   也可以再次运行 `install.bat`：此时它不会再要求重启，而是直接执行自检，
   确认驱动已就绪。

卸载：双击 `uninstall.bat`（从驱动仓库删除驱动包、修好键盘类 `UpperFilters`、
并从用户 `PATH` 中移除本目录）。

> **关于签名**：发布包中的驱动使用自签名**测试证书**，因此安装时会开启 Windows
> 测试签名模式（`bcdedit /set testsigning on`）。这是个人/开发场景的常规做法。
> 若要面向公众正式分发，需要购买 EV 代码签名证书并通过微软硬件开发者中心签名。

---

## 架构

```
用户态 (Ring 3)
└── Go 控制程序 (keyboardctl.exe)
    └── 业务逻辑、IOCTL 通信 → \\.\KbdFilter

内核态 (Ring 0)
└── C 键盘过滤驱动 (keyboardsimulator.sys, KMDF 上过滤驱动)
    ├── DriverEntry：WDF 驱动 + 控制设备 \\.\KbdFilter（供用户态通信）
    ├── EvtDeviceAdd：为每个键盘创建一个过滤设备，挂在 kbdclass 之下
    └── 拦截 IOCTL_INTERNAL_KEYBOARD_CONNECT，截获 kbdclass 的回调用于注入
```

本驱动是**键盘设备类的上过滤驱动**（class upper filter）。安装程序把它写进键盘类的
`UpperFilters`，且排在 `kbdclass` **之前**，于是每个键盘的栈变成：

```
kbdclass -> keyboardsimulator -> <端口驱动 i8042prt / kbdhid> -> <总线>
```

之所以必须位于 `kbdclass` 之下：`kbdclass` 会向下发送
`IOCTL_INTERNAL_KEYBOARD_CONNECT`，该请求携带 `kbdclass` 自己的
`KeyboardClassServiceCallback`。驱动保存这个回调，再把自己的回调交给端口驱动，
于是真实按键先到达本驱动。注入时复用同一个回调：把合成的 `KEYBOARD_INPUT_DATA`
交给 `kbdclass`，其处理方式与端口驱动上报硬件事件完全相同。

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
                      │  IOCTL_INTERNAL_KEYBOARD_CONNECT（向下）
                      ▼
┌─────────────────────────────────────────────────────┐
│     keyboardsimulator.sys (Ring 0, KMDF 上过滤)     │
│     截获 ClassService，转发真实按键                 │
│     控制设备 \\Device\\KbdFilter                    │
│     符号链接 \\.\KbdFilter                          │
└─────────────────────┬───────────────────────────────┘
                      │  ① IOCTL_KEYBOARD_INJECT (DeviceIoControl)
                      │  ② IOCTL_KEYBOARD_QUERY_STATE (状态查询)
                      ▼
┌─────────────────────────────────────────────────────┐
│     keyboardctl.exe (Ring 3, Go)                    │
│     -key / -text / -combo / -hold / -release        │
└─────────────────────────────────────────────────────┘
```

### 内核驱动 (`driver/`)

| 文件 | 用途 |
|------|------|
| `driver.h` | 共享定义：`DEVICE_EXTENSION`、`GLOBAL_CONTEXT`、IOCTL 码、函数原型 |
| `driver.c` | `DriverEntry`（WDF 驱动 + 控制设备）、`EvtDeviceAdd`（过滤设备）、`EvtDeviceCleanup` |
| `inject.c` | `EvtIoInternalDeviceControl`（拦截 CONNECT）、`ServiceCallback`、`InjectKeyboardEvents` |
| `queue.c`  | `ControlQueueInitialize`、`EvtIoDeviceControl` — 控制设备 IOCTL 分发 |
| `keyboardctl.inf` | 驱动安装信息（原始设备/类上过滤，注册服务 + 写入 `UpperFilters`） |
| `driver.vcxproj` | MSBuild 工程（VS 2022 + WDK） |

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

```
IOCTL_KEYBOARD_QUERY_STATE = CTL_CODE(FILE_DEVICE_KEYBOARD, 0x801,
                                      METHOD_BUFFERED, FILE_READ_ACCESS)
                           = 0x000B6004
```

无输入缓冲区，输出 `KEYBOARDCTL_STATE`：

```c
typedef struct _KEYBOARDCTL_STATE {
    ULONG Connected;      // 1 = 已有键盘连接，注入会被系统接收
    ULONG KeyboardCount;  // 曾连接过的键盘数量
} KEYBOARDCTL_STATE;
```

`-selftest` 用它区分"驱动已加载"和"键盘已连接、注入真正可用"。设备可以一打开就
成功，但注入只有在 `kbdclass` 向下发送 `IOCTL_INTERNAL_KEYBOARD_CONNECT`（即过滤
驱动已正确挂在键盘栈中）之后才会生效。

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

**普通用户（下载即用）**

| 组件 | 要求 |
|------|------|
| 操作系统 | Windows 10 / 11 x64 |
| 权限 | 管理员（安装/卸载驱动时需要） |
| 其他 | 无 —— 无需安装 Go 或 WDK |

**从源码构建（开发者）**

| 组件 | 要求 |
|------|------|
| Go | ≥ 1.23（构建 CLI） |
| Visual Studio 2022 | 含“使用 C++ 的桌面开发”工作负载（构建驱动） |
| WDK | Windows Driver Kit for Windows 11（构建驱动） |

> **驱动签名**：开发阶段用 `bcdedit /set testsigning on` 开启测试签名模式
> （`install.bat` 会自动完成）。自 Windows 10 1507 起，若启用了内存完整性
> （HVCI），未签名的驱动无法加载，因此发布流程会对驱动做**测试签名**并随包附带证书。

---

## 构建（开发者）

### 从源码一键安装

在源码目录双击 `install.bat`。除了下载包所做的安装步骤外，它还会在缺少
`keyboardctl.exe` 且已安装 Go 时自动编译 CLI。

### 构建驱动（需 VS 2022 + WDK）

```bat
make driver VERSION=1.2.3
:: 等价于：
:: msbuild driver\driver.vcxproj /p:Configuration=Release /p:Platform=x64 /p:DriverVersion=1.2.3
```

输出：`driver\x64\Release\keyboardsimulator.sys`，同目录下还有 stampinf 处理过的
`keyboardctl.inf` 和已签名的目录文件 `keyboardctl.cat`（INF 里声明了
`CatalogFile`，`pnputil` 安装时需要它）。

> 旧版 WDK 的 `build -cZ` 工具已不再随当前 WDK 发布，请使用 MSBuild。

### 构建 Go CLI

```bat
make build VERSION=1.2.3   :: 输出 build\keyboardctl.exe（版本号写入二进制）
make test                  :: 运行单元测试
make help                  :: 查看所有可用目标
```

> 需要 Windows 上的 GNU make。也可以直接调用 Go：
> `go build -o build\keyboardctl.exe .\cmd\keyboardctl`

---

## 快速开始（Windows，管理员权限）

```bat
:: 1. 安装（双击，或管理员终端运行）
install.bat

:: 2. 重启（过滤驱动需在开机时挂入键盘栈）
::    重启后打开新终端

:: 3. 自检：确认驱动可达且键盘已连接
keyboardctl -selftest

:: 4. 按下 A
keyboardctl -key a

:: 5. 输入文本，按键间隔 100ms
keyboardctl -text "Hello, World!" -delay 100

:: 6. Ctrl+C
keyboardctl -combo ctrl c

:: 7. Alt+F4
keyboardctl -combo lalt f4

:: 8. 查看版本
keyboardctl -version

:: 9. 完成后卸载驱动（之后需重启生效）
uninstall.bat
```

---

## CLI 参考

```
keyboardctl <key>                           等价于 -key <key>（简写）
keyboardctl -key    <key>                   按下并释放一个键
keyboardctl -hold   <key>                   仅按下（key-down）
keyboardctl -release <key>                  仅释放（key-up）
keyboardctl -combo  <modifier> <key>        组合键（如 Ctrl+C）
keyboardctl -text   <string> [-delay <ms>]  逐字符输入字符串
keyboardctl -list                           列出所有可用的按键名称
keyboardctl -selftest                       检查驱动可达且键盘已连接（不发送按键）
keyboardctl -version                        打印版本并退出
```

按键参数：十六进制扫描码（`0x1E`）或不区分大小写的名称（`a`、`enter`、`ctrl`、`f12`、`up` 等）。
运行 `keyboardctl -list` 可列出全部名称（含别名）。

> `-text` / `-combo` 只能产生 **US 键盘布局**下的字符。其它字符（重音字母、中文、
> emoji 等）没有对应扫描码，`-text` 会先打印一条警告并跳过它们。

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
│   └── driver.vcxproj       # MSBuild 工程（VS + WDK）
├── cmd/
│   └── keyboardctl/         # Go CLI + 库
│       ├── main.go          # CLI 入口
│       ├── driver.go        # KeyboardDriver: Open/Close/SendKey/sendBatch
│       ├── keyboard.go      # PressKey/ReleaseKey/TapKey/TypeText/PressModifierAndKey
│       ├── scancodes.go     # 扫描码常量 + 字符映射
│       └── scancodes_test.go # 单元测试
├── .changeset/              # changesets 配置与变更记录
├── .github/
│   ├── workflows/ci.yml     # CI：格式化 / vet / 测试 / 构建 CLI 与驱动
│   └── workflows/release.yml# 发布：版本 PR + 构建 + 签名 + 打包 + Release
├── install.bat              # 一键安装（提权 + 证书 + INF 安装 + UpperFilters + PATH）
├── uninstall.bat            # 驱动卸载（删除驱动包 + 修复 UpperFilters）
├── Makefile
├── package.json             # 版本号来源（changesets 管理）
├── CHANGELOG.md             # 由 changesets 自动生成
├── go.mod
├── LICENSE                  # GPL-2.0
└── README.md
```

---

## 发布流程

版本号与变更日志由 [changesets](https://github.com/changesets/changesets) 管理：

1. 每个有行为变化的 PR 运行 `npm run changeset` 添加一条变更记录。
2. 合并进 `main` 后，CI 自动开/更新一个 **`chore: version packages`** PR，
   它会消费变更记录、递增 `package.json` 版本并写入 `CHANGELOG.md`。
3. 合并该 PR 会触发发布任务：编译驱动与 CLI、用测试证书签名驱动、打包 zip，
   并创建 GitHub Release（tag 形如 `v1.2.3`）。

产物 zip 内含：`keyboardctl.exe`、`keyboardsimulator.sys`、`keyboardctl.inf`、
`keyboardctl.cat`、`keyboardctl-test.cer`、`install.bat`、`uninstall.bat`、
`README.md`、`LICENSE`。

---

## 许可证

GPL-2.0，详见 [LICENSE](LICENSE)。
