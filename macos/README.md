# InfTask macOS 原生预览

独立的 SwiftUI + 少量 AppKit 外壳，不使用 WebView。用于决定原生迁移的视觉方向，**不是完整客户端，也不承诺当前像素级一致**。

## 构建与打开

前提：macOS 14+、Xcode 15+（含 Swift 5.9+ 命令行工具）。不依赖 Go 服务或 npm；在仓库根目录执行：

```sh
swift build --package-path macos -c release
bash macos/package.sh
open "macos/dist/InfTask Preview.app"
```

打包脚本输出本机架构的 `.app` 和 `InfTask-Preview-macOS-<架构>.zip`，均在被 Git 忽略的 `macos/dist/` 下。也可双击 `.app`，或手动拖入 Applications。当前仅做 ad-hoc 签名，没有 Developer ID、公证、自动更新或 Universal Binary；从网络分发到其他机器可能被 Gatekeeper 阻止，不能作为正式发行包。

## 可以体验

- 232pt 侧栏、分组列表、水平滚动看板、详情与属性面板、标签展示。
- 状态和 Project 筛选、即时搜索、列表分组折叠、List / Board 切换。
- 点击任务进入详情，点击返回或 Escape 返回；Command-F 聚焦搜索。
- 窗口缩放（最小 980 × 640）、跟随系统浅色/深色外观。
- 内置 Inter Variable 字体，原生绘制状态、优先级和导航图标。

## 明确不做

所有 `DEMO-*` 任务都是虚构的内存常量。应用没有网络调用，不启动后端，不打开数据库，不读取维护者实例。不提供新建、编辑、保存、评论提交、标签管理、拖拽、真实 Markdown 渲染、云端登录或多 workspace 切换。关闭后不恢复筛选与浏览位置。

搜索在此预览中即时匹配，和 Web 的 Enter 提交不同。保留主要设计参数，但没有复制全部 Web 控件与布局；标题栏采用 macOS 原生窗口行为，部分图标几何简化。详情中的 checklist 和 activity 是预置展示，不表示真实任务完成情况。

## 资源来源

- 颜色、字体尺寸和业务状态图形参考 `web/src/app.css` 与 `web/src/shared.tsx`；仅维护预览所需的 token 子集。
- `Sources/InfTaskPreview/Resources/InterVariable.ttf` 来自 [Inter v4.1](https://github.com/rsms/inter/tree/v4.1)，SIL Open Font License 1.1，许可证随包附带。
- 导航图标参考当前 Web 使用的 Lucide 图形，以 SwiftUI Path 绘制，部分几何简化；ISC 许可证随包附带。不加载或执行 JavaScript。
- 应用图标是奶油底色的橙色任务卡片；可编辑源文件为 [`assets/InfTask.icon`](../assets/InfTask.icon)，由 Icon Composer 制作。`Sources/InfTaskPreview/Resources/AppIcon.png` 是其 1024px 导出，`make-icon.swift` 加上 macOS 留白后生成 Dock 所需的各尺寸。侧栏标识和活动头像保持原样。导出方式见[设计系统](../docs/DESIGN_SYSTEM.md#应用标识)。

## 验证

```sh
bash -n macos/package.sh
bash macos/package.sh
codesign --verify --deep --strict "macos/dist/InfTask Preview.app"
open "macos/dist/InfTask Preview.app"
```

构建和签名通过不证明交互正确。需人工检查筛选、搜索、空结果、分组折叠、列表/看板、详情返回、窗口缩放与两种外观；当前没有原生 UI 自动化测试。
