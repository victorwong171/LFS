# LFS - Local File Storage

[简体中文](#简体中文) | [English](#english)

---

# 简体中文

🚀 **高性能大文件存储服务** - 支持分片上传、断点续传、完整性校验的单可执行文件服务。

## ✨ 特性

### 🎯 核心功能
- **大文件分片上传** - 支持超大文件的分片传输与并发。
- **断点续传** - 网络中断后可继续上传，避免重复传输。
- **双重校验机制** - 同时支持**真实内容MD5**和**高速元数据复合MD5**（`fileName:size:lastModified`），保证高安全性与极速上传的最佳平衡。
- **插拔式设计** - MD5校验与计算功能完全插件化，可通过配置文件灵活开启或关闭，解耦核心存储逻辑。
- **批量操作** - 支持批量多文件并发上传与批量打包下载。
- **静态资源嵌入** - 酷炫的极客暗黑风前端界面，完全编译嵌入至单个Go可执行文件中。

### ⚡ 性能与优化
- **HTTP/2支持** - 多路复用，大幅减少高并发下的连接开销。
- **流式Gzip压缩** - 原生流式Gzip压缩，静态资源压缩率达73%，无额外内存开销。
- **智能内存缓存** - 静态文件全内存缓存，Etag强一致验证，零磁盘I/O延迟。
- **高并发连接池** - 支持100并发上传，200并发下载。
- **大缓冲区读写** - 4MB传输缓冲区与2MB分片缓冲区，极大减少磁盘开销。
- **异步MD5计算** - 采用64MB恒定分块与3并发信号量限制，支持任意大小文件计算而不阻塞列表与API。

### 🛡️ 安全特性
- **CORS支持** - 灵活的安全跨域访问控制。
- **安全防护** - XSS保护、内容类型嗅探防御。
- **路径沙箱机制** - 严格的路径合法性校验，防止一切越权和路径遍历攻击。

---

## 🚀 快速开始

### 构建和运行

```bash
# 克隆项目
git clone git@github.com:victorwong171/LFS.git
cd LFS

# 构建（生成单个可执行文件）
go build -o bin/lfs-server.exe ./cmd/lfs-server

# 运行
./bin/lfs-server.exe
```

服务将在 `http://localhost:8080` 启动。

### 配置文件 `config.json`

在程序运行目录下创建 `config.json`：

```json
{
  "storage_path": "./data",
  "enable_md5": true
}
```

- `storage_path`: 文件存储的目标目录。
- `enable_md5`: 是否开启MD5完整性校验（如果设为`false`，所有上传将绕过MD5验证并立即保存，计算任务将通过Null接口隔离）。

---

## 📡 API 接口

### 文件上传
```bash
# 单文件上传
curl -X POST -F "file=@example.txt" http://localhost:8080/upload

# 分片上传
curl -X POST -F "file=@chunk.bin" \
  -F "fileName=large_file.bin" \
  -F "totalSize=52428800" \
  -F "chunkIndex=0" \
  -F "chunkSize=5242880" \
  -F "totalChunk=10" \
  -F "md5=abc123" \
  -F "modTime=1780000000" \
  http://localhost:8080/upload-chunk

# 批量上传
curl -X POST -F "files=@file1.txt" -F "files=@file2.txt" \
  http://localhost:8080/batch-upload
```

### 文件下载与管理
```bash
# 单文件下载
curl -O http://localhost:8080/download/example.txt

# 分片下载
curl "http://localhost:8080/download-chunk/example.txt?chunkIndex=0&chunkSize=5242880"

# 批量下载
curl "http://localhost:8080/batch-download?filenames=file1.txt,file2.txt"

# 获取文件列表（异步自动计算MD5）
curl http://localhost:8080/files
```

---

# English

🚀 **High-Performance Large File Storage Service** - A single executable file server supporting chunked uploads, resumable transfers, integrity verification, and embedded UI.

## ✨ Features

### 🎯 Core Functions
- **Large File Chunked Upload** - Supports multi-threaded parallel chunked transfers for extremely large files.
- **Resumable Transfers** - Seamlessly resumes uploads after network interruptions, avoiding duplicate transfers.
- **Dual Validation Mechanism** - Supports both **Real Content MD5** and **High-Speed Metadata Composite MD5** (`fileName:size:lastModified`) to achieve the perfect balance between high security and fast uploads.
- **Pluggable Architecture** - The MD5 validation and async calculation features are completely modularized and pluggable. You can easily enable/disable them via `config.json` to decouple the core storage logic.
- **Batch Operations** - Supports concurrent batch file uploads and packaged batch downloads.
- **Embedded UI** - Featuring a modern dark geek-style web dashboard, fully compiled and embedded into a single Go executable.

### ⚡ Performance & Optimization
- **HTTP/2 Support** - Multiplexing drastically cuts connection overhead under high concurrency.
- **Streaming Gzip Compression** - Native streaming Gzip compression yields a 73% compression ratio for static assets with zero memory footprints.
- **Smart Memory Caching** - Full in-memory static file cache, ETag strong consistency verification, and zero disk I/O latency.
- **High-Concurrency Connection Pools** - Confirmed support for 100 concurrent uploads and 200 concurrent downloads.
- **Large Buffer Processing** - 4MB transfer buffer and 2MB chunk buffer significantly decrease disk read/write cycles.
- **Asynchronous MD5 Calculations** - 64MB constant chunk size and a 3-concurrency semaphore limit allow hashing of files of any size without blocking main API responses.

### 🛡️ Security
- **CORS Support** - Flexible cross-origin resource sharing access controls.
- **Security Protections** - XSS protection and Content-Type sniffing defense.
- **Path Sandboxing** - Strict path validation checks to prevent directory traversal and unauthorized directory access.

---

## 🚀 Quick Start

### Build and Run

```bash
# Clone the repository
git clone git@github.com:victorwong171/LFS.git
cd LFS

# Build (Produces a single executable binary)
go build -o bin/lfs-server.exe ./cmd/lfs-server

# Run
./bin/lfs-server.exe
```

The server will spin up at `http://localhost:8080`.

### Configuration File `config.json`

Create a `config.json` file in the same directory as the executable:

```json
{
  "storage_path": "./data",
  "enable_md5": true
}
```

- `storage_path`: Target directory for storing files.
- `enable_md5`: Toggle MD5 integrity validation. If set to `false`, all uploads will bypass MD5 validation, skipping background hashing completely through clean Null adapters.

---

## 📡 API Endpoints

### File Uploads
```bash
# Single File Upload
curl -X POST -F "file=@example.txt" http://localhost:8080/upload

# Chunked Upload
curl -X POST -F "file=@chunk.bin" \
  -F "fileName=large_file.bin" \
  -F "totalSize=52428800" \
  -F "chunkIndex=0" \
  -F "chunkSize=5242880" \
  -F "totalChunk=10" \
  -F "md5=abc123" \
  -F "modTime=1780000000" \
  http://localhost:8080/upload-chunk

# Batch Upload
curl -X POST -F "files=@file1.txt" -F "files=@file2.txt" \
  http://localhost:8080/batch-upload
```

### Downloads & Management
```bash
# Single File Download
curl -O http://localhost:8080/download/example.txt

# Chunked Download
curl "http://localhost:8080/download-chunk/example.txt?chunkIndex=0&chunkSize=5242880"

# Batch Download
curl "http://localhost:8080/batch-download?filenames=file1.txt,file2.txt"

# List Files (Asynchronously computes missing MD5 hashes)
curl http://localhost:8080/files
```