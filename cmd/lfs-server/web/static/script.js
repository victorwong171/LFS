/* ==========================================================================
   LFS - Core Client Logic
   ========================================================================== */

// DOM 元素引用
const dropArea = document.getElementById('drop-area');
const fileInput = document.getElementById('file-input');
const selectFilesBtn = document.getElementById('select-files-btn');
const uploadButton = document.getElementById('upload-button');
const queueList = document.getElementById('upload-queue-list');
const fileTreeContainer = document.getElementById('file-tree-container');
const refreshListButton = document.getElementById('refresh-list');
const batchDownloadBtn = document.getElementById('batch-download-btn');
const searchInput = document.getElementById('search-input');

// 聊天室 DOM 元素
const chatMessages = document.getElementById('chat-messages');
const chatInput = document.getElementById('chat-input');
const chatSendBtn = document.getElementById('chat-send-btn');
const chatStatus = document.getElementById('chat-status');
const nicknameDisplay = document.getElementById('nickname-display');

// 模态框 DOM 元素
const nicknameModal = document.getElementById('nickname-modal');
const modalNicknameInput = document.getElementById('modal-nickname-input');
const modalCancelBtn = document.getElementById('modal-cancel-btn');
const modalSaveBtn = document.getElementById('modal-save-btn');

// 全局状态
let ws = null;
let uploadQueue = []; // 存储 UploadTask 实例的队列
let currentFilesData = []; // 存储当前渲染的文件树数据，方便搜索过滤
let userNickname = "";

// 酷炫极客随机昵称生成词库
const coolAdjectives = ["Quantum", "Cyber", "Hyper", "Alpha", "Nova", "Cosmic", "Shadow", "Vector", "Crypto", "Nebula", "Helix", "Pixel"];
const coolNouns = ["Hacker", "Explorer", "Coder", "Runner", "Ninja", "Rider", "Pilot", "Specter", "Matrix", "Photon", "Pioneer", "Giga"];

function generateRandomNickname() {
    const adj = coolAdjectives[Math.floor(Math.random() * coolAdjectives.length)];
    const noun = coolNouns[Math.floor(Math.random() * coolNouns.length)];
    const num = Math.floor(Math.random() * 90) + 10;
    return `${adj}-${noun}-${num}`;
}

// 初始化昵称
function initNickname() {
    const saved = localStorage.getItem('lfs_nickname');
    if (saved && saved.trim()) {
        userNickname = saved;
    } else {
        userNickname = generateRandomNickname();
        localStorage.setItem('lfs_nickname', userNickname);
    }
    nicknameDisplay.textContent = userNickname;
}

// 模态框昵称修改控制
document.getElementById('change-nickname-btn').addEventListener('click', () => {
    modalNicknameInput.value = userNickname;
    nicknameModal.classList.add('active');
    modalNicknameInput.focus();
});

modalCancelBtn.addEventListener('click', () => {
    nicknameModal.classList.remove('active');
});

modalSaveBtn.addEventListener('click', () => {
    const val = modalNicknameInput.value.trim();
    if (val) {
        userNickname = val;
        localStorage.setItem('lfs_nickname', userNickname);
        nicknameDisplay.textContent = userNickname;
        
        // 昵称改变时如果连着 WebSocket，通知后端（发送一个特殊空消息或直接改变前端显示即可）
        // 这里项目后端只解析 msg.message 并结合连接的 IP 昵称，所以直接本地生效，之后发消息就带新昵称
    }
    nicknameModal.classList.remove('active');
});

// 点击模态框背景关闭
nicknameModal.addEventListener('click', (e) => {
    if (e.target === nicknameModal) {
        nicknameModal.classList.remove('active');
    }
});

/* ==========================================================================
   Upload Task 面向对象传输管理器
   ========================================================================== */
class UploadTask {
    constructor(file) {
        this.id = 'task_' + Date.now() + '_' + Math.random().toString(36).substr(2, 9);
        this.file = file;
        this.name = file.name;
        this.size = file.size;
        
        // 大文件界限（10MB）
        this.isLarge = this.size > 10 * 1024 * 1024;
        this.chunkSize = 10 * 1024 * 1024; // 升级为 10MB 分片，大幅减少 HTTP 请求往返与网络开销，解决性能瓶颈
        this.totalChunks = this.isLarge ? Math.ceil(this.size / this.chunkSize) : 1;
        this.currentChunk = 0;
        
        // 传输进度与速率相关
        this.status = 'ready'; // 'ready' | 'hashing' | 'uploading' | 'paused' | 'success' | 'error'
        this.progress = 0;
        this.speed = 0; // 字节/秒
        this.eta = 0; // 秒
        
        // Fast composite MD5 key based on filename, size and modification time (in seconds)
        const modTime = Math.floor(this.file.lastModified / 1000) || 0;
        this.modTime = modTime;
        this.md5 = md5(`${this.name}:${this.size}:${modTime}`);
        
        // 网络句柄
        this.xhr = null;
        
        // 速率测算变量
        this.uploadedBytes = 0;
        this.lastUploadedBytes = 0;
        this.lastTime = 0;
        this.speedInterval = null;
        
        // 渲染在 UI 上的 DOM 引用
        this.domElement = null;
    }

    // 在 UI 中创建或获取对应的卡片
    renderCard() {
        const ext = this.name.split('.').pop().toLowerCase();
        let iconClass = 'fa-file';
        if (['zip', 'rar', 'tar', 'gz', '7z'].includes(ext)) iconClass = 'fa-file-zipper';
        else if (['mp4', 'mkv', 'avi', 'mov'].includes(ext)) iconClass = 'fa-file-video';
        else if (['mp3', 'wav', 'flac', 'ogg'].includes(ext)) iconClass = 'fa-file-audio';
        else if (['jpg', 'jpeg', 'png', 'gif', 'svg', 'webp'].includes(ext)) iconClass = 'fa-file-image';
        else if (['pdf', 'doc', 'docx', 'xls', 'xlsx', 'ppt', 'pptx', 'txt'].includes(ext)) iconClass = 'fa-file-lines';

        let actionButtons = '';
        if (this.status === 'uploading' && this.isLarge) {
            actionButtons = `<button class="task-btn btn-pause" title="暂停"><i class="fas fa-pause"></i></button>`;
        } else if (this.status === 'paused' && this.isLarge) {
            actionButtons = `<button class="task-btn btn-resume" title="继续"><i class="fas fa-play"></i></button>`;
        }
        
        if (this.status !== 'success' && this.status !== 'error') {
            actionButtons += `<button class="task-btn btn-cancel" title="取消"><i class="fas fa-xmark"></i></button>`;
        } else {
            actionButtons = `<button class="task-btn btn-remove-done" title="清除卡片"><i class="fas fa-trash-can"></i></button>`;
        }

        // 计算速率和 ETA 展示文本
        let speedText = '';
        let etaText = '';
        if (this.status === 'uploading') {
            speedText = formatSpeed(this.speed);
            etaText = this.eta > 0 ? `剩余 ${formatTime(this.eta)}` : '计算中...';
        } else if (this.status === 'hashing') {
            speedText = '计算 MD5 校验中...';
        } else if (this.status === 'paused') {
            speedText = '已暂停';
        } else if (this.status === 'success') {
            speedText = '上传成功';
        } else if (this.status === 'error') {
            speedText = '上传失败';
        } else if (this.status === 'ready') {
            speedText = '等待中';
        }

        const statusClass = this.status === 'success' ? 'success' : (this.status === 'error' ? 'error' : (this.status === 'hashing' ? 'hashing' : ''));

        if (!this.domElement) {
            const card = document.createElement('div');
            card.className = 'upload-task-card';
            card.id = this.id;
            this.domElement = card;
            
            // 首次渲染：构建完整的 DOM 框架
            card.innerHTML = `
                <div class="task-card-row">
                    <div class="task-info">
                        <i class="file-type-icon fas ${iconClass}"></i>
                        <div class="task-meta">
                            <span class="task-filename" title="${this.name}">${this.name}</span>
                            <span class="task-size">${formatFileSize(this.size)}</span>
                        </div>
                    </div>
                    <div class="task-actions">
                        ${actionButtons}
                    </div>
                </div>
                <div class="task-progress-wrapper">
                    <div class="task-progress-track">
                        <div class="task-progress-bar ${statusClass}" style="width: ${this.progress}%"></div>
                    </div>
                    <div class="task-progress-details">
                        <span class="task-percent">${Math.round(this.progress)}%</span>
                        <div class="task-speed-eta">
                            <span class="task-speed-text">${speedText}</span>
                            <span class="task-eta-text">${etaText}</span>
                        </div>
                    </div>
                </div>
            `;
            queueList.appendChild(card);
        } else {
            // 增量式直接更新，保留 CSS transition 平滑动画，拒绝 flickering
            const progressBar = this.domElement.querySelector('.task-progress-bar');
            const percentText = this.domElement.querySelector('.task-percent');
            const speedSpan = this.domElement.querySelector('.task-speed-text');
            const etaSpan = this.domElement.querySelector('.task-eta-text');
            const actionsDiv = this.domElement.querySelector('.task-actions');

            if (progressBar) {
                progressBar.className = `task-progress-bar ${statusClass}`;
                progressBar.style.width = this.progress + '%';
            }
            if (percentText) {
                percentText.textContent = Math.round(this.progress) + '%';
            }
            if (speedSpan) {
                speedSpan.textContent = speedText;
            }
            if (etaSpan) {
                etaSpan.textContent = etaText;
            }
            if (actionsDiv) {
                actionsDiv.innerHTML = actionButtons;
            }
        }

        // 绑定卡片上的按钮事件
        this.bindEvents();
    }

    bindEvents() {
        const pauseBtn = this.domElement.querySelector('.btn-pause');
        if (pauseBtn) pauseBtn.onclick = () => this.pause();

        const resumeBtn = this.domElement.querySelector('.btn-resume');
        if (resumeBtn) resumeBtn.onclick = () => this.resume();

        const cancelBtn = this.domElement.querySelector('.btn-cancel');
        if (cancelBtn) cancelBtn.onclick = () => this.cancel();

        const removeBtn = this.domElement.querySelector('.btn-remove-done');
        if (removeBtn) {
            removeBtn.onclick = () => {
                this.destroy();
            };
        }
    }

    // 启动测速计时器
    startSpeedTracking() {
        this.lastTime = Date.now();
        this.lastUploadedBytes = this.uploadedBytes;
        
        this.speedInterval = setInterval(() => {
            const now = Date.now();
            const timePassed = (now - this.lastTime) / 1000; // 秒
            if (timePassed <= 0) return;
            
            const bytesAdded = this.uploadedBytes - this.lastUploadedBytes;
            this.speed = bytesAdded / timePassed; // 字节/秒
            
            const remaining = this.size - this.uploadedBytes;
            this.eta = this.speed > 0 ? (remaining / this.speed) : 0;
            
            this.lastTime = now;
            this.lastUploadedBytes = this.uploadedBytes;
            
            this.renderCard();
        }, 1000);
    }

    stopSpeedTracking() {
        if (this.speedInterval) {
            clearInterval(this.speedInterval);
            this.speedInterval = null;
        }
        this.speed = 0;
        this.eta = 0;
    }

    // 开始上传入口
    start() {
        if (this.status !== 'ready' && this.status !== 'error') return;
        
        // 移除 Placeholder
        const placeholder = queueList.querySelector('.empty-queue-placeholder');
        if (placeholder) placeholder.style.display = 'none';

        this.status = 'uploading';
        this.renderCard();
        this.startSpeedTracking();

        if (this.isLarge) {
            this.uploadLarge();
        } else {
            this.uploadSmall();
        }
    }

    // 小文件直传 (batch-upload)
    uploadSmall() {
        const formData = new FormData();
        formData.append('files', this.file);

        this.xhr = new XMLHttpRequest();
        this.xhr.open('POST', '/batch-upload', true);

        // 监听进度
        this.xhr.upload.onprogress = (e) => {
            if (e.lengthComputable) {
                this.uploadedBytes = e.loaded;
                this.progress = (e.loaded / e.total) * 100;
                this.renderCard();
            }
        };

        this.xhr.onreadystatechange = () => {
            if (this.xhr.readyState === 4) {
                this.stopSpeedTracking();
                if (this.xhr.status === 200) {
                    this.status = 'success';
                    this.progress = 100;
                    this.renderCard();
                    onTaskFinished();
                } else {
                    this.status = 'error';
                    this.renderCard();
                    showNotification(`文件 ${this.name} 上传失败`, 'error');
                }
            }
        };

        this.xhr.onerror = () => {
            this.stopSpeedTracking();
            this.status = 'error';
            this.renderCard();
            showNotification(`网络错误，${this.name} 上传失败`, 'error');
        };

        this.xhr.send(formData);
    }

    // 大文件分片流式增量计算与上传 (On-the-fly Hashing & Uploading)
    uploadLarge() {
        const uploadNext = () => {
            if (this.status !== 'uploading') return;
            
            const start = this.currentChunk * this.chunkSize;
            const end = Math.min(start + this.chunkSize, this.size);
            const chunkBlob = this.file.slice(start, end);

            // Determine if this is the last chunk
            let chunkMD5 = "STREAMING_DUMMY_MD5";
            if (this.currentChunk === this.totalChunks - 1) {
                chunkMD5 = this.md5; // Send the precalculated composite MD5 on the last chunk
            }

            // 准备分片表单数据
            const formData = new FormData();
            formData.append('fileName', this.name);
            formData.append('totalSize', this.size);
            formData.append('chunkIndex', this.currentChunk);
            formData.append('chunkSize', chunkBlob.size);
            formData.append('totalChunk', this.totalChunks);
            formData.append('md5', chunkMD5);
            formData.append('modTime', this.modTime || 0);
            formData.append('file', chunkBlob, `${this.name}.part${this.currentChunk}`);

            this.xhr = new XMLHttpRequest();
            this.xhr.open('POST', '/upload-chunk', true);

            // 监听此分片进度
            this.xhr.upload.onprogress = (progressEvent) => {
                if (progressEvent.lengthComputable && this.status === 'uploading') {
                    const chunkLoaded = progressEvent.loaded;
                    const totalBytesSent = start + chunkLoaded;
                    this.uploadedBytes = Math.min(totalBytesSent, this.size);
                    
                    // 综合大进度百分比
                    const chunkPercent = chunkLoaded / progressEvent.total;
                    this.progress = ((this.currentChunk + chunkPercent) / this.totalChunks) * 100;
                    this.renderCard();
                }
            };
            this.xhr.onreadystatechange = () => {
                if (this.xhr.readyState === 4) {
                    if (this.xhr.status === 200) {
                        this.currentChunk++;
                        
                        // 核心修复：显式更新整体大进度，并调用 renderCard()！
                        // 这确保了即使在高宽带环境下，XHR onprogress 没有被浏览器高频触发，进度条也能随着每一个分片的合拢稳步前行，彻底解决进度条不动的缺陷！
                        this.progress = (this.currentChunk / this.totalChunks) * 100;
                        this.uploadedBytes = Math.min(this.currentChunk * this.chunkSize, this.size);
                        this.renderCard();

                        if (this.currentChunk < this.totalChunks) {
                            uploadNext(); // 接着投递下一片
                        } else {
                            // 全部完成
                            this.stopSpeedTracking();
                            this.status = 'success';
                            this.progress = 100;
                            this.renderCard();
                            showNotification(`大文件 ${this.name} 上传成功！`, 'success');
                            onTaskFinished();
                        }
                    } else {
                        this.stopSpeedTracking();
                        this.status = 'error';
                        this.renderCard();
                        let errMsg = "分片上传失败";
                        try {
                            const resp = JSON.parse(this.xhr.responseText);
                            if (resp.error) errMsg = resp.error;
                        } catch(e) {}
                        showNotification(`${this.name} 上传中断: ${errMsg}`, 'error');
                    }
                }
            };

            this.xhr.onerror = () => {
                this.stopSpeedTracking();
                this.status = 'error';
                this.renderCard();
                showNotification(`大文件分片网络链接异常`, 'error');
            };

            this.xhr.send(formData);
        };

        // 发起首片循环
        uploadNext();
    }

    // 暂停分片
    pause() {
        if (this.status !== 'uploading' || !this.isLarge) return;
        this.status = 'paused';
        this.stopSpeedTracking();
        if (this.xhr) {
            this.xhr.abort(); // 中断当前在传的 XHR
        }
        this.renderCard();
        showNotification(`文件 ${this.name} 上传已暂停`, 'info');
    }

    // 恢复分片继续上传
    resume() {
        if (this.status !== 'paused' || !this.isLarge) return;
        this.status = 'uploading';
        this.renderCard();
        this.startSpeedTracking();
        this.uploadLarge(); // 自动从 currentChunk 继续
    }

    // 取消传输
    cancel() {
        this.status = 'error';
        this.stopSpeedTracking();
        if (this.xhr) {
            this.xhr.abort();
        }
        showNotification(`已取消上传 ${this.name}`, 'info');
        this.destroy();
    }

    // 彻底销毁
    destroy() {
        this.stopSpeedTracking();
        if (this.domElement) {
            this.domElement.remove();
        }
        // 从队列移出
        uploadQueue = uploadQueue.filter(t => t.id !== this.id);
        
        // 如果队列空了，复现 Placeholder
        if (uploadQueue.length === 0) {
            const placeholder = queueList.querySelector('.empty-queue-placeholder');
            if (placeholder) placeholder.style.display = 'flex';
            uploadButton.disabled = true;
        }
    }
}

// 格式化测速展示
function formatSpeed(bytesPerSec) {
    if (bytesPerSec === 0 || isNaN(bytesPerSec)) return '0 B/s';
    const k = 1024;
    const sizes = ['B/s', 'KB/s', 'MB/s', 'GB/s'];
    const i = Math.floor(Math.log(bytesPerSec) / Math.log(k));
    return parseFloat((bytesPerSec / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i];
}

// 格式化剩余时间
function formatTime(seconds) {
    if (seconds === 0 || isNaN(seconds) || seconds === Infinity) return '0s';
    if (seconds < 60) return Math.round(seconds) + 's';
    const mins = Math.floor(seconds / 60);
    const secs = Math.round(seconds % 60);
    return `${mins}m ${secs}s`;
}

// 单任务完成时刷新列表
function onTaskFinished() {
    // 检查队列中是不是都上传完了
    const activeTasks = uploadQueue.filter(t => t.status === 'uploading' || t.status === 'hashing');
    if (activeTasks.length === 0) {
        fetchFileList(); // 全部传完刷新列表
    }
}

/* ==========================================================================
   选择与拖拽事件逻辑
   ========================================================================== */

// 阻止默认拖放事件，使得拖拽区域生效
['dragenter', 'dragover', 'dragleave', 'drop'].forEach(eventName => {
    dropArea.addEventListener(eventName, preventDefaults, false);
    document.body.addEventListener(eventName, preventDefaults, false);
});

function preventDefaults(e) {
    e.preventDefault();
    e.stopPropagation();
}

['dragenter', 'dragover'].forEach(eventName => {
    dropArea.addEventListener(eventName, () => dropArea.classList.add('highlight'), false);
});

['dragleave', 'drop'].forEach(eventName => {
    dropArea.addEventListener(eventName, () => dropArea.classList.remove('highlight'), false);
});

// 处理拖入释放
dropArea.addEventListener('drop', (e) => {
    const dt = e.dataTransfer;
    const files = dt.files;
    addFilesToQueue(files);
});

// 处理按钮文件点选
selectFilesBtn.addEventListener('click', () => fileInput.click());
fileInput.addEventListener('change', () => {
    addFilesToQueue(fileInput.files);
});

// 将选择的文件压入独立任务管理器队列
function addFilesToQueue(files) {
    const arr = Array.from(files);
    if (arr.length === 0) return;

    // 移除 Placeholder
    const placeholder = queueList.querySelector('.empty-queue-placeholder');
    if (placeholder) placeholder.style.display = 'none';

    arr.forEach(file => {
        // 防止同个文件重复推入队列
        if (uploadQueue.some(t => t.name === file.name && t.size === file.size)) {
            return;
        }
        
        const task = new UploadTask(file);
        uploadQueue.push(task);
        task.renderCard();
    });

    uploadButton.disabled = false;
}

// 队列总起动按钮
uploadButton.addEventListener('click', () => {
    uploadQueue.forEach(task => {
        if (task.status === 'ready' || task.status === 'error') {
            task.start();
        }
    });
});

/* ==========================================================================
   文件浏览器操作与渲染过滤
   ========================================================================== */

// 递归对文件树列表按修改时间降序排序（最新修改/上传的文件夹或文件排在最上面）
function sortFilesByModTime(files) {
    if (!files || files.length === 0) return [];
    
    // 递归对每个子文件夹的内容进行排序
    files.forEach(file => {
        if (file.is_dir && file.children && file.children.length > 0) {
            sortFilesByModTime(file.children);
        }
    });
    
    // 核心优化：纯时间降序，新上传或新修改的文件/文件夹绝对置顶在最上方，无视“目录优先”限制以确保新上传最突出
    return files.sort((a, b) => {
        const timeA = a.mod_time ? new Date(a.mod_time).getTime() : 0;
        const timeB = b.mod_time ? new Date(b.mod_time).getTime() : 0;
        return timeB - timeA;
    });
}

// 获取文件列表并加载渲染
function fetchFileList() {
    const xhr = new XMLHttpRequest();
    xhr.open('GET', '/files', true);
    
    xhr.onreadystatechange = function () {
        if (xhr.readyState === 4) {
            if (xhr.status === 200) {
                try {
                    const response = JSON.parse(xhr.responseText);
                    currentFilesData = response.files || [];
                    
                    // 极致性能体验：渲染前按修改时间降序排序（新上传的文件/文件夹置顶）
                    sortFilesByModTime(currentFilesData);
                    
                    renderFileTree(currentFilesData);
                } catch (error) {
                    fileTreeContainer.innerHTML = '<div class="empty-list">解析文件树失败</div>';
                }
            } else {
                fileTreeContainer.innerHTML = '<div class="empty-list">服务器连接失败</div>';
            }
        }
    };
    
    xhr.onerror = function() {
        fileTreeContainer.innerHTML = '<div class="empty-list">网络异常，无法获取文件</div>';
    };
    
    xhr.send();
}

// 渲染文件树（顶层）
function renderFileTree(files) {
    fileTreeContainer.innerHTML = '';
    
    if (!files || files.length === 0) {
        fileTreeContainer.innerHTML = '<div class="empty-list"><i class="fas fa-circle-info"></i> 暂无文件，请先在左侧上传</div>';
        return;
    }
    
    const tree = document.createElement('ul');
    tree.className = 'file-tree';
    
    files.forEach(file => {
        const item = createFileTreeItem(file);
        tree.appendChild(item);
    });
    
    fileTreeContainer.appendChild(tree);
    
    // 重置批量下载状态
    updateBatchDownloadBtnState();
}

// 递归创建文件树节点 DOM
function createFileTreeItem(file) {
    const li = document.createElement('li');
    li.className = `file-tree-item ${file.is_dir ? 'folder' : 'file'}`;
    if (!file.is_dir) {
        const ext = file.name.split('.').pop().toLowerCase();
        li.setAttribute('data-ext', ext);
    }
    
    // 外层包装元素，用于悬停特效与高亮
    const wrapper = document.createElement('div');
    wrapper.className = 'file-tree-item-wrapper';
    
    // 多选复选框 (仅针对文件，文件夹暂不支持多选)
    if (!file.is_dir) {
        const checkbox = document.createElement('input');
        checkbox.type = 'checkbox';
        checkbox.className = 'file-select-checkbox';
        checkbox.value = file.path;
        checkbox.addEventListener('change', (e) => {
            e.stopPropagation();
            updateBatchDownloadBtnState();
        });
        wrapper.appendChild(checkbox);
    } else {
        // 文件夹占位占个空框，保证对齐
        const emptyBox = document.createElement('div');
        emptyBox.style.width = '17px';
        wrapper.appendChild(emptyBox);
    }
    
    const icon = document.createElement('i');
    icon.className = `file-tree-item-icon fas ${file.is_dir ? 'fa-folder' : 'fa-file'}`;
    
    const nameSpan = document.createElement('span');
    nameSpan.className = 'file-tree-item-name';
    nameSpan.textContent = file.name;
    
    const infoSpan = document.createElement('span');
    infoSpan.className = 'file-tree-item-info';
    
    if (!file.is_dir) {
        infoSpan.textContent = `${formatFileSize(file.size)}`;
        if (file.md5) {
            infoSpan.title = `MD5: ${file.md5}`;
            // 鼠标移上去可以显示全 MD5
        }
    }
    
    wrapper.appendChild(icon);
    wrapper.appendChild(nameSpan);
    wrapper.appendChild(infoSpan);
    
    // 添加下载/删除操作
    const actions = document.createElement('div');
    actions.className = 'file-tree-item-actions';
    
    if (!file.is_dir) {
        const dlLink = document.createElement('a');
        dlLink.href = `/download/${encodeURIComponent(file.path)}`;
        dlLink.className = 'file-tree-action-link';
        dlLink.innerHTML = '<i class="fas fa-cloud-arrow-down"></i>';
        dlLink.title = '下载此文件';
        dlLink.addEventListener('click', (e) => e.stopPropagation()); // 防止触发折叠事件
        actions.appendChild(dlLink);
    }
    wrapper.appendChild(actions);
    li.appendChild(wrapper);
    
    // 如果是文件夹且有子项，递归渲染
    if (file.is_dir && file.children && file.children.length > 0) {
        const childrenUl = document.createElement('ul');
        childrenUl.className = 'file-tree-children';
        
        file.children.forEach(child => {
            const childItem = createFileTreeItem(child);
            childrenUl.appendChild(childItem);
        });
        
        // 文件夹点击伸缩逻辑
        let expanded = false;
        wrapper.addEventListener('click', function(e) {
            e.stopPropagation();
            expanded = !expanded;
            if (expanded) {
                childrenUl.style.display = 'block';
                icon.className = 'file-tree-item-icon fas fa-folder-open';
            } else {
                childrenUl.style.display = 'none';
                icon.className = 'file-tree-item-icon fas fa-folder';
            }
        });
        
        li.appendChild(childrenUl);
    }
    
    return li;
}

// 格式化文件大小
function formatFileSize(bytes) {
    if (bytes === 0) return '0 B';
    const k = 1024;
    const sizes = ['B', 'KB', 'MB', 'GB'];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i];
}

// 纯前端快速检索过滤（基于当前渲染的节点 DOM）
searchInput.addEventListener('input', () => {
    const val = searchInput.value.toLowerCase().trim();
    const items = fileTreeContainer.querySelectorAll('.file-tree-item.file');
    const folders = fileTreeContainer.querySelectorAll('.file-tree-item.folder');
    
    if (!val) {
        // 如果清空，全部复原
        fileTreeContainer.querySelectorAll('.file-tree-item').forEach(el => {
            el.style.display = '';
        });
        fileTreeContainer.querySelectorAll('.file-tree-children').forEach(el => {
            el.style.display = 'none'; // 折叠回去
        });
        fileTreeContainer.querySelectorAll('.file-tree-item-icon').forEach(icon => {
            if (icon.classList.contains('fa-folder-open')) {
                icon.className = 'file-tree-item-icon fas fa-folder';
            }
        });
        return;
    }
    
    // 第一步：先隐藏全部文件夹，显示有匹配的文件，并一路往上展开其父类容器
    folders.forEach(f => f.style.display = 'none');
    
    items.forEach(item => {
        const name = item.querySelector('.file-tree-item-name').textContent.toLowerCase();
        if (name.includes(val)) {
            item.style.display = '';
            
            // 逐层往上把父容器展开并设置为显示状态
            let parent = item.parentElement;
            while (parent && parent !== fileTreeContainer) {
                if (parent.classList.contains('file-tree-children')) {
                    parent.style.display = 'block'; // 强行展开
                }
                if (parent.classList.contains('file-tree-item') && parent.classList.contains('folder')) {
                    parent.style.display = ''; // 强行显示文件夹
                    const fIcon = parent.querySelector('.file-tree-item-icon');
                    if (fIcon) fIcon.className = 'file-tree-item-icon fas fa-folder-open';
                }
                parent = parent.parentElement;
            }
        } else {
            item.style.display = 'none';
        }
    });
});

// 批量下载状态机维护
function updateBatchDownloadBtnState() {
    const checked = getCheckedFiles();
    if (checked.length > 0) {
        batchDownloadBtn.disabled = false;
        batchDownloadBtn.innerHTML = `<i class="fas fa-cloud-arrow-down"></i> 批量下载 (${checked.length})`;
    } else {
        batchDownloadBtn.disabled = true;
        batchDownloadBtn.innerHTML = `<i class="fas fa-download"></i> 批量下载`;
    }
}

function getCheckedFiles() {
    const checkboxes = fileTreeContainer.querySelectorAll('.file-select-checkbox:checked');
    return Array.from(checkboxes).map(cb => cb.value);
}

// 批量下载流水线触发
batchDownloadBtn.addEventListener('click', () => {
    const paths = getCheckedFiles();
    if (paths.length === 0) return;
    
    showNotification(`已拉起 ${paths.length} 个下载任务，请允许浏览器的多文件下载...`, 'info');

    // 采用 iframe 注入或虚拟 a 标签点击触发多任务串行下载，规避浏览器安全拦截
    paths.forEach((p, idx) => {
        setTimeout(() => {
            const dlLink = document.createElement('a');
            dlLink.href = `/download/${encodeURIComponent(p)}`;
            dlLink.download = p.split('/').pop();
            dlLink.style.display = 'none';
            document.body.appendChild(dlLink);
            dlLink.click();
            document.body.removeChild(dlLink);
        }, idx * 400); // 间隔 400ms 触发一次，防卡死
    });
});

/* ==========================================================================
   WebSocket 极客聊天室
   ========================================================================== */

function initWebSocket() {
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    const wsUrl = `${protocol}//${window.location.host}/ws/chat`;
    
    ws = new WebSocket(wsUrl);
    
    ws.onopen = function() {
        chatStatus.textContent = '已连接';
        chatStatus.className = 'chat-status connected';
        chatInput.disabled = false;
        chatSendBtn.disabled = false;
        
        // 连接建立后，瞬间向服务器发送初始化昵称包，更新服务器端的 client.nickname
        ws.send(JSON.stringify({
            type: 'init',
            nickname: userNickname,
            message: ''
        }));
    };
    
    ws.onclose = function() {
        chatStatus.textContent = '已断开';
        chatStatus.className = 'chat-status disconnected';
        chatInput.disabled = true;
        chatSendBtn.disabled = true;
        
        // 5秒后自动尝试重连
        setTimeout(initWebSocket, 5000);
    };
    
    ws.onerror = function() {
        chatStatus.textContent = '连接错误';
        chatStatus.className = 'chat-status disconnected';
    };
    
    ws.onmessage = function(event) {
        const lines = event.data.trim().split('\n').filter(line => line.trim());
        for (let line of lines) {
            try {
                const message = JSON.parse(line);
                if (message && typeof message === 'object') {
                    if (!message.type) message.type = 'message';
                    if (message.message === undefined) message.message = '';
                    addChatMessage(message);
                }
            } catch (error) {
                // 静默处理错误包
            }
        }
    };
}

// 聊天室气泡添加与智能滚动
function addChatMessage(message) {
    if (!message) return;
    
    const isSelf = message.nickname === userNickname;
    const msgType = message.type || 'message';
    
    const bubble = document.createElement('div');
    bubble.className = `chat-message ${msgType} ${isSelf ? 'self' : ''}`;
    
    if (msgType === 'message') {
        const header = document.createElement('div');
        header.className = 'chat-msg-header';
        
        const nickSpan = document.createElement('span');
        nickSpan.className = 'chat-msg-nickname';
        nickSpan.textContent = message.nickname || '匿名极客';
        header.appendChild(nickSpan);
        
        const metaSpan = document.createElement('span');
        metaSpan.className = 'chat-msg-meta';
        
        // 展示精简 IP (例如 [192.168.1.100]) 和时间
        let metaText = "";
        if (message.ip) metaText += `[${message.ip}] `;
        if (message.timestamp) metaText += `${message.timestamp.split(' ')[1] || message.timestamp}`; // 仅显示时分秒
        metaSpan.textContent = metaText.trim();
        header.appendChild(metaSpan);
        
        const content = document.createElement('div');
        content.className = 'chat-msg-content';
        content.textContent = message.message || '';
        
        bubble.appendChild(header);
        bubble.appendChild(content);
    } else {
        // join/leave 通知
        bubble.textContent = message.message || '';
    }
    
    // 智能滚动置底控制
    const isNearBottom = chatMessages.scrollHeight - chatMessages.clientHeight - chatMessages.scrollTop < 60;
    
    chatMessages.appendChild(bubble);
    
    if (isNearBottom || isSelf) {
        chatMessages.scrollTop = chatMessages.scrollHeight;
    }
}

// 发送消息
function sendChatMessage() {
    const messageText = chatInput.value.trim();
    if (messageText && ws && ws.readyState === WebSocket.OPEN) {
        ws.send(JSON.stringify({
            type: 'message',
            nickname: userNickname, // 附加当前极客昵称到请求包里
            message: messageText
        }));
        chatInput.value = '';
    }
}

chatInput.addEventListener('keypress', (e) => {
    if (e.key === 'Enter') sendChatMessage();
});

chatSendBtn.addEventListener('click', sendChatMessage);

/* ==========================================================================
   通知与状态框渲染
   ========================================================================== */
const statusDiv = document.getElementById('status');

function showNotification(message, type = 'info') {
    statusDiv.innerHTML = `<div class="${type}-message">${message}</div>`;
    setTimeout(() => {
        statusDiv.innerHTML = '';
    }, 4000);
}

/* ==========================================================================
   DOM 加载与初始化起动
   ========================================================================== */
document.addEventListener('DOMContentLoaded', () => {
    initNickname();
    fetchFileList();
    initWebSocket();
    initSystemControl(); // 初始化系统控制面板
});

refreshListButton.addEventListener('click', fetchFileList);

// ==========================================================================
// Zero-Dependency Pure JavaScript MD5 Implementation
// ==========================================================================
function md5(string) {
    function rotateLeft(lValue, iShiftBits) {
        return (lValue << iShiftBits) | (lValue >>> (32 - iShiftBits));
    }
    function addUnsigned(lX, lY) {
        var lX4, lY4, lX8, lY8, lResult;
        lX8 = (lX & 0x80000000);
        lY8 = (lY & 0x80000000);
        lX4 = (lX & 0x40000000);
        lY4 = (lY & 0x40000000);
        lResult = (lX & 0x3FFFFFFF) + (lY & 0x3FFFFFFF);
        if (lX4 & lY4) {
            return (lResult ^ 0x80000000 ^ lX8 ^ lY8);
        }
        if (lX4 | lY4) {
            if (lResult & 0x40000000) {
                return (lResult ^ 0xC0000000 ^ lX8 ^ lY8);
            } else {
                return (lResult ^ 0x40000000 ^ lX8 ^ lY8);
            }
        } else {
            return (lResult ^ lX8 ^ lY8);
        }
    }
    function F(x, y, z) { return (x & y) | ((~x) & z); }
    function G(x, y, z) { return (x & z) | (y & (~z)); }
    function H(x, y, z) { return (x ^ y ^ z); }
    function I(x, y, z) { return (y ^ (x | (~z))); }
    function FF(a, b, c, d, x, s, ac) {
        a = addUnsigned(a, addUnsigned(addUnsigned(F(b, c, d), x), ac));
        return addUnsigned(rotateLeft(a, s), b);
    }
    function GG(a, b, c, d, x, s, ac) {
        a = addUnsigned(a, addUnsigned(addUnsigned(G(b, c, d), x), ac));
        return addUnsigned(rotateLeft(a, s), b);
    }
    function HH(a, b, c, d, x, s, ac) {
        a = addUnsigned(a, addUnsigned(addUnsigned(H(b, c, d), x), ac));
        return addUnsigned(rotateLeft(a, s), b);
    }
    function II(a, b, c, d, x, s, ac) {
        a = addUnsigned(a, addUnsigned(addUnsigned(I(b, c, d), x), ac));
        return addUnsigned(rotateLeft(a, s), b);
    }
    function convertToWordArray(string) {
        var lWordCount;
        var lMessageLength = string.length;
        var lNumberOfWords_temp1 = lMessageLength + 8;
        var lNumberOfWords_temp2 = (lNumberOfWords_temp1 - (lNumberOfWords_temp1 % 64)) / 64;
        var lNumberOfWords = (lNumberOfWords_temp2 + 1) * 16;
        var lWordArray = Array(lNumberOfWords);
        var lBytePosition = 0;
        var lByteCount = 0;
        while (lByteCount < lMessageLength) {
            lWordCount = (lByteCount - (lByteCount % 4)) / 4;
            lBytePosition = (lByteCount % 4) * 8;
            lWordArray[lWordCount] = (lWordArray[lWordCount] | (string.charCodeAt(lByteCount) << lBytePosition));
            lByteCount++;
        }
        lWordCount = (lByteCount - (lByteCount % 4)) / 4;
        lBytePosition = (lByteCount % 4) * 8;
        lWordArray[lWordCount] = lWordArray[lWordCount] | (0x80 << lBytePosition);
        lWordArray[lNumberOfWords - 2] = lMessageLength << 3;
        lWordArray[lNumberOfWords - 1] = lMessageLength >>> 29;
        return lWordArray;
    }
    function wordToHex(lValue) {
        var WordToHexValue = "", WordToHexValue_temp = "", lByte, lCount;
        for (lCount = 0; lCount <= 3; lCount++) {
            lByte = (lValue >>> (lCount * 8)) & 255;
            WordToHexValue_temp = "0" + lByte.toString(16);
            WordToHexValue = WordToHexValue + WordToHexValue_temp.substr(WordToHexValue_temp.length - 2, 2);
        }
        return WordToHexValue;
    }
    function utf8Encode(string) {
        string = string.replace(/\r\n/g, "\n");
        var utftext = "";
        for (var n = 0; n < string.length; n++) {
            var c = string.charCodeAt(n);
            if (c < 128) {
                utftext += String.fromCharCode(c);
            } else if ((c > 127) && (c < 2048)) {
                utftext += String.fromCharCode((c >> 6) | 192);
                utftext += String.fromCharCode((c & 63) | 128);
            } else {
                utftext += String.fromCharCode((c >> 12) | 224);
                utftext += String.fromCharCode(((c >> 6) & 63) | 128);
                utftext += String.fromCharCode((c & 63) | 128);
            }
        }
        return utftext;
    }
    var x = Array();
    var k, S11, S12, S13, S14, S21, S22, S23, S24, S31, S32, S33, S34, S41, S42, S43, S44;
    var a = 0x67452301; var b = 0xEFCDAB89; var c = 0x98BADCFE; var d = 0x10325476;
    string = utf8Encode(string);
    x = convertToWordArray(string);
    S11 = 7; S12 = 12; S13 = 17; S14 = 22;
    S21 = 5; S22 = 9; S23 = 14; S24 = 20;
    S31 = 4; S32 = 11; S33 = 16; S34 = 23;
    S41 = 6; S42 = 10; S43 = 15; S44 = 21;
    for (k = 0; k < x.length; k += 16) {
        var AA = a; var BB = b; var CC = c; var DD = d;
        a = FF(a, b, c, d, x[k + 0], S11, 0xD76AA478); d = FF(d, a, b, c, x[k + 1], S12, 0xE8C7B756);
        c = FF(c, d, a, b, x[k + 2], S13, 0x242070DB); b = FF(b, c, d, a, x[k + 3], S14, 0xC1BDCEEE);
        a = FF(a, b, c, d, x[k + 4], S11, 0xF57C0FAF); d = FF(d, a, b, c, x[k + 5], S12, 0x4787C62A);
        c = FF(c, d, a, b, x[k + 6], S13, 0xA8304613); b = FF(b, c, d, a, x[k + 7], S14, 0xFD469501);
        a = FF(a, b, c, d, x[k + 8], S11, 0x698098D8); d = FF(d, a, b, c, x[k + 9], S12, 0x8B44F7AF);
        c = FF(c, d, a, b, x[k + 10], S13, 0xFFFF5BB1); b = FF(b, c, d, a, x[k + 11], S14, 0x895CD7BE);
        a = FF(a, b, c, d, x[k + 12], S11, 0x6B901122); d = FF(d, a, b, c, x[k + 13], S12, 0xFD987193);
        c = FF(c, d, a, b, x[k + 14], S13, 0xA679438E); b = FF(b, c, d, a, x[k + 15], S14, 0x49B40821);
        a = GG(a, b, c, d, x[k + 1], S21, 0xF61E2562); d = GG(d, a, b, c, x[k + 6], S22, 0xC040B340);
        c = GG(c, d, a, b, x[k + 11], S23, 0x265E5A51); b = GG(b, c, d, a, x[k + 0], S24, 0xE9B6C7AA);
        a = GG(a, b, c, d, x[k + 5], S21, 0xD62F105D); d = GG(d, a, b, c, x[k + 10], S22, 0x02441453);
        c = GG(c, d, a, b, x[k + 15], S23, 0xD8A1E681); b = GG(b, c, d, a, x[k + 4], S24, 0xE7D3FBC8);
        a = GG(a, b, c, d, x[k + 9], S21, 0x21E1CDE6); d = GG(d, a, b, c, x[k + 14], S22, 0xC33707D6);
        c = GG(c, d, a, b, x[k + 3], S23, 0xF4D50D87); b = GG(b, c, d, a, x[k + 8], S24, 0x455A14ED);
        a = GG(a, b, c, d, x[k + 13], S21, 0xA9E3E905); d = GG(d, a, b, c, x[k + 2], S22, 0xFCEFA3F8);
        c = GG(c, d, a, b, x[k + 7], S23, 0x676F02D9); b = GG(b, c, d, a, x[k + 12], S24, 0x8D2A4C8A);
        a = HH(a, b, c, d, x[k + 5], S31, 0xFFFA3942); d = HH(d, a, b, c, x[k + 8], S32, 0x8771F681);
        c = HH(c, d, a, b, x[k + 11], S33, 0x6D9D6122); b = HH(b, c, d, a, x[k + 14], S34, 0xFDE5380C);
        a = HH(a, b, c, d, x[k + 1], S31, 0xA4BEEA44); d = HH(d, a, b, c, x[k + 4], S32, 0x4BDECFA9);
        c = HH(c, d, a, b, x[k + 7], S33, 0xF6BB4B60); b = HH(b, c, d, a, x[k + 10], S34, 0xBEBFBC70);
        a = HH(a, b, c, d, x[k + 13], S31, 0x289B7EC6); d = HH(d, a, b, c, x[k + 0], S32, 0xEAA127FA);
        c = HH(c, d, a, b, x[k + 3], S33, 0xD4EF3085); b = HH(b, c, d, a, x[k + 6], S34, 0x04881D05);
        a = HH(a, b, c, d, x[k + 9], S31, 0xD9D4D039); d = HH(d, a, b, c, x[k + 12], S32, 0xE6DB99E5);
        c = HH(c, d, a, b, x[k + 15], S33, 0x1FA27CF8); b = HH(b, c, d, a, x[k + 2], S34, 0xC4AC5665);
        a = II(a, b, c, d, x[k + 0], S41, 0xF4292244); d = II(d, a, b, c, x[k + 7], S42, 0x432AFF97);
        c = II(c, d, a, b, x[k + 14], S43, 0xAB9423A7); b = II(b, c, d, a, x[k + 5], S44, 0xFC93A039);
        a = II(a, b, c, d, x[k + 12], S41, 0x655B59C3); d = II(d, a, b, c, x[k + 3], S42, 0x8F0CCC92);
        c = II(c, d, a, b, x[k + 10], S43, 0xFFEFF47D); b = II(b, c, d, a, x[k + 1], S44, 0x85845DD1);
        a = II(a, b, c, d, x[k + 8], S41, 0x6FA87E4F); d = II(d, a, b, c, x[k + 15], S42, 0xFE2CE6E0);
        c = II(c, d, a, b, x[k + 6], S43, 0xA3014314); b = II(b, c, d, a, x[k + 13], S44, 0x4E0811A1);
        a = II(a, b, c, d, x[k + 4], S41, 0xF7537E82); d = II(d, a, b, c, x[k + 11], S42, 0xBD3AF235);
        c = II(c, d, a, b, x[k + 2], S43, 0x2AD7D2BB); b = II(b, c, d, a, x[k + 9], S44, 0xEB86D391);
        a = addUnsigned(a, AA); b = addUnsigned(b, BB); c = addUnsigned(c, CC); d = addUnsigned(d, DD);
    }
    var temp = wordToHex(a) + wordToHex(b) + wordToHex(c) + wordToHex(d);
    return temp.toLowerCase();
}

// ==========================================================================
// 宿主机系统控制面板客户端逻辑
// ==========================================================================
function initSystemControl() {
    // DOM 元素绑定
    const heartbeat = document.getElementById('system-heartbeat');
    const transfersVal = document.getElementById('sys-active-transfers');
    const shutdownModeVal = document.getElementById('sys-shutdown-mode');
    const displayStateVal = document.getElementById('sys-display-state');
    const countdownBar = document.getElementById('shutdown-countdown-bar');
    const countdownText = document.getElementById('countdown-text');
    
    const modeSelect = document.getElementById('shutdown-mode-select');
    const delayContainer = document.getElementById('shutdown-delay-container');
    const delayInput = document.getElementById('shutdown-delay-input');
    const delayUnit = document.getElementById('shutdown-delay-unit');
    
    const btnTrigger = document.getElementById('btn-trigger-shutdown');
    const btnCancel = document.getElementById('btn-cancel-shutdown');
    const btnDispOff = document.getElementById('btn-display-off');
    const btnDispOn = document.getElementById('btn-display-on');

    if (!heartbeat) return; // 防御性判断

    // 根据选择的关机模式，显示或隐藏延迟时间输入框
    modeSelect.addEventListener('change', () => {
        if (modeSelect.value === 'scheduled') {
            delayContainer.style.display = 'flex';
        } else {
            delayContainer.style.display = 'none';
        }
    });

    // 触发关机计划
    btnTrigger.addEventListener('click', () => {
        const mode = modeSelect.value;
        let delay = 0;
        if (mode === 'scheduled') {
            const val = parseInt(delayInput.value);
            const unit = parseInt(delayUnit.value);
            if (isNaN(val) || val <= 0) {
                showNotification('请输入有效的定时关机时间！', 'error');
                return;
            }
            delay = val * unit;
        }

        // 二次确认，防止误触立即关机
        if (mode === 'immediate') {
            if (!confirm('您确定要立即关闭宿主机系统吗？未完成的任务将会中断。')) {
                return;
            }
        }

        const xhr = new XMLHttpRequest();
        xhr.open('POST', '/system/shutdown', true);
        xhr.setRequestHeader('Content-Type', 'application/json');
        xhr.onreadystatechange = () => {
            if (xhr.readyState === 4) {
                if (xhr.status === 200) {
                    showNotification('关机计划已成功提交', 'success');
                    updateSystemStatus();
                } else {
                    let errMsg = '提交关机计划失败';
                    try {
                        const res = JSON.parse(xhr.responseText);
                        if (res.error) errMsg = res.error;
                    } catch(e) {}
                    showNotification(errMsg, 'error');
                }
            }
        };
        xhr.send(JSON.stringify({ mode: mode, delay: delay }));
    });

    // 取消关机计划
    btnCancel.addEventListener('click', () => {
        const xhr = new XMLHttpRequest();
        xhr.open('POST', '/system/shutdown/cancel', true);
        xhr.onreadystatechange = () => {
            if (xhr.readyState === 4) {
                if (xhr.status === 200) {
                    showNotification('关机计划已取消', 'success');
                    updateSystemStatus();
                } else {
                    showNotification('取消关机计划失败', 'error');
                }
            }
        };
        xhr.send();
    });

    // 息屏
    btnDispOff.addEventListener('click', () => {
        setDisplayState('off');
    });

    // 唤醒屏幕
    btnDispOn.addEventListener('click', () => {
        setDisplayState('on');
    });

    function setDisplayState(state) {
        const xhr = new XMLHttpRequest();
        xhr.open('POST', '/system/display', true);
        xhr.setRequestHeader('Content-Type', 'application/json');
        xhr.onreadystatechange = () => {
            if (xhr.readyState === 4) {
                if (xhr.status === 200) {
                    showNotification(state === 'on' ? '屏幕唤醒指令已发送' : '屏幕息屏指令已发送', 'success');
                    updateSystemStatus();
                } else {
                    showNotification('控制显示器状态失败', 'error');
                }
            }
        };
        xhr.send(JSON.stringify({ state: state }));
    }

    // 定时获取并更新系统状态
    function updateSystemStatus() {
        const xhr = new XMLHttpRequest();
        xhr.open('GET', '/system/status', true);
        xhr.onreadystatechange = () => {
            if (xhr.readyState === 4) {
                if (xhr.status === 200) {
                    try {
                        const status = JSON.parse(xhr.responseText);
                        heartbeat.classList.remove('error');
                        heartbeat.title = '系统服务在线';

                        // 1. 活跃传输数
                        transfersVal.textContent = status.active_transfers;

                        // 2. 显示器状态
                        displayStateVal.textContent = status.display_on ? '已开启' : '已息屏';

                        // 3. 关机模式与倒计时
                        let modeText = '无计划';
                        btnCancel.style.display = 'none';
                        countdownBar.style.display = 'none';

                        if (status.shutdown_mode === 'on_complete') {
                            modeText = '任务完关机';
                            btnCancel.style.display = 'block';
                        } else if (status.shutdown_mode === 'immediate') {
                            modeText = '即时关机';
                        } else if (status.shutdown_mode === 'scheduled') {
                            modeText = '倒计时关机';
                            btnCancel.style.display = 'block';
                            
                            // 渲染倒计时时间
                            if (status.shutdown_time) {
                                const targetTime = new Date(status.shutdown_time).getTime();
                                const diff = Math.max(0, Math.floor((targetTime - Date.now()) / 1000));
                                if (diff > 0) {
                                    const min = Math.floor(diff / 60);
                                    const sec = diff % 60;
                                    countdownText.innerHTML = `<i class="fas fa-hourglass-half"></i> 系统将于 <strong>${min}分${sec}秒</strong> 后关闭`;
                                } else {
                                    countdownText.innerHTML = `<i class="fas fa-hourglass-half"></i> 系统正在关机...`;
                                }
                                countdownBar.style.display = 'block';
                            }
                        }
                        shutdownModeVal.textContent = modeText;
                    } catch(e) {
                        handleStatusError();
                    }
                } else {
                    handleStatusError();
                }
            }
        };
        xhr.send();
    }

    function handleStatusError() {
        heartbeat.classList.add('error');
        heartbeat.title = '连接断开';
        transfersVal.textContent = '--';
        shutdownModeVal.textContent = '--';
        displayStateVal.textContent = '--';
        countdownBar.style.display = 'none';
        btnCancel.style.display = 'none';
    }

    // 初始执行一次并设置每秒轮询（为了倒计时秒数实时刷新）
    updateSystemStatus();
    setInterval(updateSystemStatus, 1000);
}