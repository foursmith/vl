# 构建

- 构建可执行文件时使用 `make build`，输出到 `bin/elegit`。
- 用户通过 `~/.local/bin/elegit` 的符号链接运行此仓库的 `bin/elegit`，因此交付需要运行验证的修改时，必须更新这个构建产物。
- 不要用 `go build ./cmd/elegit` 代替交付构建；它会在仓库根目录生成 `elegit`，不会更新用户实际运行的文件。
- 已运行的 Elegit 进程需要退出后重新打开，才会使用新的构建。
