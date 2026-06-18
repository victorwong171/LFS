#!/bin/bash

# 大文件MD5计算测试脚本

echo "🚀 测试大文件MD5计算支持"
echo "================================"

# 创建测试目录
mkdir -p test_files

# 创建不同大小的测试文件
echo "📁 创建测试文件..."

# 小文件 (1MB)
echo "创建 1MB 文件..."
dd if=/dev/zero of=test_files/small_1mb.bin bs=1M count=1 2>/dev/null

# 中等文件 (100MB)
echo "创建 100MB 文件..."
dd if=/dev/zero of=test_files/medium_100mb.bin bs=1M count=100 2>/dev/null

# 大文件 (1GB) - 如果磁盘空间允许
echo "创建 1GB 文件..."
dd if=/dev/zero of=test_files/large_1gb.bin bs=1M count=1024 2>/dev/null

# 超大文件 (10GB) - 如果磁盘空间允许
echo "创建 10GB 文件..."
dd if=/dev/zero of=test_files/huge_10gb.bin bs=1M count=10240 2>/dev/null

echo "✅ 测试文件创建完成"
echo ""

# 启动服务器（后台运行）
echo "🚀 启动服务器..."
./lfs-server &
SERVER_PID=$!

# 等待服务器启动
sleep 3

echo "📊 测试文件列表接口性能..."
echo "================================"

# 测试文件列表接口
echo "测试文件列表接口（应该立即返回）..."
time curl -s http://localhost:8080/files | jq '.[] | {name: .name, size: .size, md5: .md5}'

echo ""
echo "📈 测试MD5计算进度查询..."
echo "================================"

# 测试MD5进度查询
for file in small_1mb.bin medium_100mb.bin large_1gb.bin huge_10gb.bin; do
    if [ -f "test_files/$file" ]; then
        echo "查询 $file 的MD5计算进度..."
        curl -s "http://localhost:8080/file-md5-progress/$file" | jq .
        echo ""
    fi
done

echo "⏳ 等待MD5计算完成..."
echo "================================"

# 等待一段时间让MD5计算完成
sleep 10

echo "📋 最终文件列表（包含MD5）..."
echo "================================"
curl -s http://localhost:8080/files | jq '.[] | {name: .name, size: .size, md5: .md5}'

echo ""
echo "🧪 测试单个文件MD5获取..."
echo "================================"

# 测试单个文件MD5获取
for file in small_1mb.bin medium_100mb.bin large_1gb.bin huge_10gb.bin; do
    if [ -f "test_files/$file" ]; then
        echo "获取 $file 的MD5..."
        time curl -s "http://localhost:8080/file-md5/$file" | jq .
        echo ""
    fi
done

# 停止服务器
echo "🛑 停止服务器..."
kill $SERVER_PID

# 清理测试文件
echo "🧹 清理测试文件..."
rm -rf test_files

echo "✅ 测试完成！"
