/**
 * HexaRAG Developer Dashboard
 * Interactive functionality for the developer dashboard
 */

class DeveloperDashboard {
    constructor() {
        this.apiBase = '/api/v1';
        this.wsConnection = null;
        this.charts = {};
        this.metrics = {
            performance: [],
            messageFlow: [],
            connections: 0,
            activeConversations: 0
        };
        this.refreshInterval = null;
        
        this.init();
    }
    
    async init() {
        console.log('🔧 Initializing HexaRAG Developer Dashboard');
        
        // Initialize UI components
        this.initializeNavigation();
        this.initializeEventListeners();
        this.initializeCharts();
        
        // Start data fetching
        await this.loadInitialData();
        this.startMetricsPolling();
        this.initializeWebSocket();
        
        // Initialize scripts integration
        this.initializeScriptsIntegration();
        
        console.log('✅ Dashboard initialized successfully');
        this.addLogEntry('info', 'Dashboard initialized successfully');
    }
    
    // Navigation Management
    initializeNavigation() {
        const navTabs = document.querySelectorAll('.nav-tab');
        const tabContents = document.querySelectorAll('.tab-content');
        
        navTabs.forEach(tab => {
            tab.addEventListener('click', () => {
                const targetTab = tab.dataset.tab;
                
                // Update active tab
                navTabs.forEach(t => t.classList.remove('active'));
                tab.classList.add('active');
                
                // Update active content
                tabContents.forEach(content => {
                    content.classList.remove('active');
                });
                document.getElementById(`${targetTab}-tab`).classList.add('active');
                
                // Load tab-specific data
                this.loadTabData(targetTab);
            });
        });
    }
    
    // Event Listeners
    initializeEventListeners() {
        // Header actions
        document.getElementById('refreshBtn').addEventListener('click', () => {
            this.refreshAllData();
        });
        
        // Quick actions
        document.getElementById('quickSetupBtn').addEventListener('click', () => {
            this.runQuickSetup();
        });
        
        document.getElementById('healthCheckBtn').addEventListener('click', () => {
            this.runHealthCheck();
        });
        
        document.getElementById('benchmarkBtn').addEventListener('click', () => {
            this.runBenchmark();
        });
        
        document.getElementById('resetDbBtn').addEventListener('click', () => {
            this.showResetDbConfirmation();
        });
        
        // Models tab
        document.getElementById('refreshModelsBtn').addEventListener('click', () => {
            this.loadModels();
        });
        
        document.getElementById('downloadModelBtn').addEventListener('click', () => {
            this.showDownloadModelDialog();
        });
        
        // Console
        document.getElementById('consoleInput').addEventListener('keypress', (e) => {
            if (e.key === 'Enter') {
                this.executeCommand();
            }
        });
        
        document.getElementById('executeCommandBtn').addEventListener('click', () => {
            this.executeCommand();
        });
        
        document.getElementById('clearConsoleBtn').addEventListener('click', () => {
            this.clearConsole();
        });
        
        // Command suggestions
        document.querySelectorAll('.suggestion-btn').forEach(btn => {
            btn.addEventListener('click', () => {
                document.getElementById('consoleInput').value = btn.dataset.command;
            });
        });
        
        // Modal
        document.querySelectorAll('.modal-close').forEach(btn => {
            btn.addEventListener('click', () => {
                this.closeModal();
            });
        });
        
        // Logs
        document.getElementById('logLevel').addEventListener('change', () => {
            this.filterLogs();
        });
        
        document.getElementById('logSearch').addEventListener('input', () => {
            this.filterLogs();
        });
        
        document.getElementById('clearLogsBtn').addEventListener('click', () => {
            this.clearLogs();
        });
    }
    
    // Charts Initialization
    initializeCharts() {
        // Performance Chart
        const performanceCtx = document.getElementById('performanceChart').getContext('2d');
        this.charts.performance = new Chart(performanceCtx, {
            type: 'line',
            data: {
                labels: [],
                datasets: [{
                    label: 'Response Time (ms)',
                    data: [],
                    borderColor: '#1f6feb',
                    backgroundColor: 'rgba(31, 111, 235, 0.1)',
                    tension: 0.4,
                    fill: true
                }]
            },
            options: {
                responsive: true,
                maintainAspectRatio: false,
                plugins: {
                    legend: {
                        display: false
                    }
                },
                scales: {
                    x: {
                        display: false
                    },
                    y: {
                        beginAtZero: true,
                        grid: {
                            color: '#30363d'
                        },
                        ticks: {
                            color: '#8b949e'
                        }
                    }
                },
                elements: {
                    point: {
                        radius: 2
                    }
                }
            }
        });
        
        // Message Flow Chart
        const messageFlowCtx = document.getElementById('messageFlowChart').getContext('2d');
        this.charts.messageFlow = new Chart(messageFlowCtx, {
            type: 'doughnut',
            data: {
                labels: ['Sent', 'Received', 'Processing'],
                datasets: [{
                    data: [0, 0, 0],
                    backgroundColor: ['#238636', '#1f6feb', '#d29922'],
                    borderWidth: 0
                }]
            },
            options: {
                responsive: true,
                maintainAspectRatio: false,
                plugins: {
                    legend: {
                        position: 'bottom',
                        labels: {
                            color: '#8b949e',
                            font: {
                                size: 11
                            }
                        }
                    }
                }
            }
        });
    }
    
    // Data Loading
    async loadInitialData() {
        try {
            await Promise.all([
                this.loadSystemHealth(),
                this.loadPerformanceMetrics(),
                this.loadConnections(),
                this.loadModels()
            ]);
        } catch (error) {
            console.error('Failed to load initial data:', error);
            this.addLogEntry('error', `Failed to load initial data: ${error.message}`);
        }
    }
    
    async loadSystemHealth() {
        try {
            const response = await fetch(`${this.apiBase}/system/health`);
            if (!response.ok) {
                throw new Error(`HTTP ${response.status}`);
            }
            
            const health = await response.json();
            this.updateSystemHealth(health);
            this.updateStatusBar(health);
        } catch (error) {
            console.error('Failed to load system health:', error);
            this.updateStatusBar({ api: 'error', ollama: 'error', nats: 'error', database: 'error' });
        }
    }
    
    async loadPerformanceMetrics() {
        try {
            const response = await fetch(`${this.apiBase}/system/metrics`);
            if (!response.ok) {
                throw new Error(`HTTP ${response.status}`);
            }
            
            const metrics = await response.json();
            this.updatePerformanceMetrics(metrics);
        } catch (error) {
            console.error('Failed to load performance metrics:', error);
            this.setPlaceholderData();
        }
    }
    
    async loadConnections() {
        try {
            const response = await fetch(`${this.apiBase}/system/connections`);
            if (!response.ok) {
                throw new Error(`HTTP ${response.status}`);
            }
            
            const connections = await response.json();
            this.updateConnections(connections);
        } catch (error) {
            console.error('Failed to load connections:', error);
        }
    }
    
    async loadModels() {
        try {
            const response = await fetch(`${this.apiBase}/models`);
            if (!response.ok) {
                throw new Error(`HTTP ${response.status}`);
            }
            
            const data = await response.json();
            this.updateModelsGrid(data.models || []);
            this.updateModelUsage(data);
        } catch (error) {
            console.error('Failed to load models:', error);
            this.updateModelsGrid([]);
        }
    }
    
    loadTabData(tabName) {
        switch (tabName) {
            case 'overview':
                this.loadSystemHealth();
                this.loadPerformanceMetrics();
                break;
            case 'models':
                this.loadModels();
                break;
            case 'console':
                // Console is always ready
                break;
            case 'logs':
                this.loadRecentLogs();
                break;
        }
    }
    
    // UI Updates
    updateSystemHealth(health) {
        // Update system metrics
        document.getElementById('systemUptime').textContent = this.formatUptime(health.uptime || 0);
        document.getElementById('memoryUsage').textContent = health.memory_usage || '--';
        document.getElementById('cpuUsage').textContent = health.cpu_usage || '--';
        document.getElementById('requestRate').textContent = health.requests_per_minute || '0';
    }
    
    updateStatusBar(health) {
        const statusItems = {
            'apiStatus': health.api || 'error',
            'ollamaStatus': health.ollama || 'error',
            'natsStatus': health.nats || 'error',
            'dbStatus': health.database || 'error'
        };
        
        Object.entries(statusItems).forEach(([id, status]) => {
            const element = document.getElementById(id);
            element.className = `status-item ${status}`;
        });
    }
    
    updatePerformanceMetrics(metrics) {
        // Update performance stats
        document.getElementById('avgResponseTime').textContent = `${metrics.avg_response_time || 0}ms`;
        document.getElementById('p95ResponseTime').textContent = `${metrics.p95_response_time || 0}ms`;
        document.getElementById('p99ResponseTime').textContent = `${metrics.p99_response_time || 0}ms`;
        
        // Update charts with real data or mock data for demo
        this.updatePerformanceChart(metrics.response_times || this.generateMockPerformanceData());
        this.updateMessageFlowChart(metrics.message_flow || { sent: 150, received: 145, processing: 5 });
    }
    
    updateConnections(connections) {
        document.getElementById('wsConnections').textContent = connections.websocket || 0;
        document.getElementById('activeConversations').textContent = connections.active_conversations || 0;
        
        // Update connection list
        const connectionList = document.getElementById('connectionList');
        connectionList.innerHTML = '';
        
        if (connections.details && connections.details.length > 0) {
            connections.details.forEach(conn => {
                const item = document.createElement('div');
                item.className = 'connection-item';
                item.innerHTML = `
                    <span>${conn.id}</span>
                    <span>${conn.duration}</span>
                `;
                connectionList.appendChild(item);
            });
        } else {
            connectionList.innerHTML = '<div class="connection-item">No active connections</div>';
        }
    }
    
    updateModelsGrid(models) {
        const grid = document.getElementById('modelsGrid');
        grid.innerHTML = '';
        
        if (models.length === 0) {
            grid.innerHTML = '<div class="loading">No models available</div>';
            return;
        }
        
        models.forEach(model => {
            const card = document.createElement('div');
            card.className = 'model-card';
            card.innerHTML = `
                <div class="model-header">
                    <h4 class="model-title">${model.name}</h4>
                    <span class="script-status available">Available</span>
                </div>
                <div class="model-info">
                    <div class="model-detail">
                        <span class="label">Size:</span>
                        <span class="value">${this.formatBytes(model.size || 0)}</span>
                    </div>
                    <div class="model-detail">
                        <span class="label">Modified:</span>
                        <span class="value">${this.formatDate(model.modified_at)}</span>
                    </div>
                </div>
                <div class="model-actions">
                    <button class="btn btn-secondary btn-sm" onclick="dashboard.switchToModel('${model.name}')">
                        <span>🔄</span> Switch
                    </button>
                    <button class="btn btn-danger btn-sm" onclick="dashboard.deleteModel('${model.name}')">
                        <span>🗑️</span> Delete
                    </button>
                </div>
            `;
            grid.appendChild(card);
        });
    }
    
    updateModelUsage(data) {
        document.getElementById('activeModel').textContent = data.active_model || '--';
        document.getElementById('totalInferences').textContent = data.total_inferences || '0';
        document.getElementById('avgInferenceDuration').textContent = `${data.avg_inference_duration || 0}ms`;
    }
    
    updatePerformanceChart(data) {
        const chart = this.charts.performance;
        const now = new Date();
        
        // Add new data point
        chart.data.labels.push(now.toLocaleTimeString());
        chart.data.datasets[0].data.push(data[data.length - 1] || Math.random() * 1000 + 100);
        
        // Keep only last 20 points
        if (chart.data.labels.length > 20) {
            chart.data.labels.shift();
            chart.data.datasets[0].data.shift();
        }
        
        chart.update('none');
    }
    
    updateMessageFlowChart(data) {
        const chart = this.charts.messageFlow;
        chart.data.datasets[0].data = [data.sent, data.received, data.processing];
        
        // Update flow stats
        document.getElementById('totalMessages').textContent = data.sent + data.received + data.processing;
        document.getElementById('messageRate').textContent = Math.round((data.sent + data.received) / 60);
        
        chart.update();
    }
    
    setPlaceholderData() {
        // Set placeholder values when API is not available
        document.getElementById('avgResponseTime').textContent = '--ms';
        document.getElementById('p95ResponseTime').textContent = '--ms';
        document.getElementById('p99ResponseTime').textContent = '--ms';
        
        // Update charts with mock data for demo
        this.updatePerformanceChart(this.generateMockPerformanceData());
        this.updateMessageFlowChart({ sent: 0, received: 0, processing: 0 });
    }
    
    // WebSocket Connection
    initializeWebSocket() {
        try {
            const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
            const wsUrl = `${protocol}//${window.location.host}/dev/events`;
            
            this.wsConnection = new WebSocket(wsUrl);
            
            this.wsConnection.onopen = () => {
                console.log('📡 WebSocket connected');
                this.addLogEntry('info', 'WebSocket connection established');
            };
            
            this.wsConnection.onmessage = (event) => {
                try {
                    const data = JSON.parse(event.data);
                    this.handleRealtimeEvent(data);
                } catch (error) {
                    console.error('Failed to parse WebSocket message:', error);
                }
            };
            
            this.wsConnection.onclose = () => {
                console.log('📡 WebSocket disconnected');
                this.addLogEntry('warn', 'WebSocket connection lost');
                
                // Attempt to reconnect after 5 seconds
                setTimeout(() => {
                    this.initializeWebSocket();
                }, 5000);
            };
            
            this.wsConnection.onerror = (error) => {
                console.error('📡 WebSocket error:', error);
                this.addLogEntry('error', 'WebSocket connection error');
            };
        } catch (error) {
            console.error('Failed to initialize WebSocket:', error);
            this.addLogEntry('error', `Failed to initialize WebSocket: ${error.message}`);
        }
    }
    
    handleRealtimeEvent(event) {
        console.log('📡 Received real-time event:', event);
        
        switch (event.type) {
            case 'health_update':
                this.updateStatusBar(event.data);
                break;
            case 'performance_update':
                this.updatePerformanceMetrics(event.data);
                break;
            case 'connection_update':
                this.updateConnections(event.data);
                break;
            case 'model_update':
                this.loadModels();
                break;
            case 'log_entry':
                this.addLogEntry(event.data.level, event.data.message);
                break;
        }
    }
    
    // Scripts Integration
    initializeScriptsIntegration() {
        // Make script functions available globally
        window.runScript = this.runScript.bind(this);
        window.showScriptHelp = this.showScriptHelp.bind(this);
        window.dashboard = this;
    }
    
    async runScript(scriptName) {
        console.log(`🛠️ Running script: ${scriptName}`);
        this.addLogEntry('info', `Running script: ${scriptName}`);
        
        let command = '';
        let options = {};
        
        switch (scriptName) {
            case 'test-chat':
                options = {
                    model: document.getElementById('chatModel').value,
                    extended: document.getElementById('chatExtended').checked,
                    verbose: document.getElementById('chatVerbose').checked
                };
                command = this.buildChatCommand(options);
                break;
                
            case 'pull-model':
                const modelName = document.getElementById('downloadModel').value;
                if (!modelName) {
                    this.showError('Please enter a model name');
                    return;
                }
                command = `./scripts/pull-model.sh ${modelName}`;
                break;
                
            case 'list-models':
                command = './scripts/pull-model.sh --list';
                break;
                
            case 'health-check':
                options = {
                    detailed: document.getElementById('healthDetailed').checked,
                    verbose: document.getElementById('healthVerbose').checked,
                    timeout: document.getElementById('healthTimeout').value
                };
                command = this.buildHealthCommand(options);
                break;
                
            case 'benchmark':
                options = {
                    type: document.getElementById('benchmarkType').value,
                    users: document.getElementById('benchmarkUsers').value,
                    requests: document.getElementById('benchmarkRequests').value
                };
                command = this.buildBenchmarkCommand(options);
                break;
                
            case 'reset-db':
                options = {
                    backup: document.getElementById('resetBackup').checked,
                    testData: document.getElementById('resetTestData').checked,
                    force: document.getElementById('resetForce').checked
                };
                command = this.buildResetDbCommand(options);
                break;
                
            case 'dev-setup':
                options = {
                    models: document.getElementById('setupModels').checked,
                    services: document.getElementById('setupServices').checked,
                    database: document.getElementById('setupDatabase').checked
                };
                command = this.buildDevSetupCommand(options);
                break;
        }
        
        if (command) {
            await this.executeScriptCommand(command, scriptName);
        }
    }
    
    buildChatCommand(options) {
        let cmd = './scripts/test-chat.sh';
        if (options.model) cmd += ` --model ${options.model}`;
        if (options.extended) cmd += ' --extended';
        if (options.verbose) cmd += ' --verbose';
        return cmd;
    }
    
    buildHealthCommand(options) {
        let cmd = './scripts/health-check.sh';
        if (options.detailed) cmd += ' --detailed';
        if (options.verbose) cmd += ' --verbose';
        if (options.timeout) cmd += ` --timeout ${options.timeout}`;
        return cmd;
    }
    
    buildBenchmarkCommand(options) {
        let cmd = './scripts/benchmark.sh';
        if (options.type) cmd += ` --type ${options.type}`;
        if (options.users) cmd += ` --concurrent ${options.users}`;
        if (options.requests) cmd += ` --requests ${options.requests}`;
        return cmd;
    }
    
    buildResetDbCommand(options) {
        let cmd = './scripts/reset-db.sh';
        if (!options.backup) cmd += ' --no-backup';
        if (options.testData) cmd += ' --test-data';
        if (options.force) cmd += ' --force';
        return cmd;
    }
    
    buildDevSetupCommand(options) {
        let cmd = './scripts/dev-setup.sh';
        if (!options.models) cmd += ' --skip-models';
        if (!options.services) cmd += ' --skip-services';
        if (!options.database) cmd += ' --skip-database';
        cmd += ' --force'; // Always force in dashboard mode
        return cmd;
    }
    
    async executeScriptCommand(command, scriptName) {
        try {
            // Show modal with loading state
            this.showScriptModal(scriptName, 'Executing...');
            
            // Parse command to extract script and arguments
            const parts = command.split(' ');
            const script = parts[0].replace('./scripts/', '').replace('.sh', '');
            const args = parts.slice(1);
            
            // Make real API call to execute the script
            const response = await fetch('/dev/scripts/execute', {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json'
                },
                body: JSON.stringify({
                    script: script,
                    args: args
                })
            });
            
            if (!response.ok) {
                throw new Error(`HTTP ${response.status}: ${response.statusText}`);
            }
            
            const result = await response.json();
            
            // Update modal with result
            this.updateScriptModal(result.output, result.success);
            
        } catch (error) {
            console.error('Script execution failed:', error);
            this.updateScriptModal(`Error: ${error.message}`, false);
        }
    }
    
    async simulateScriptExecution(command) {
        // Simulate API call delay
        await new Promise(resolve => setTimeout(resolve, 2000));
        
        // Mock response based on command
        if (command.includes('health-check')) {
            return {
                success: true,
                output: `🏥 HexaRAG System Health Check
=============================

✓ API Server is healthy (150ms)
✓ Ollama Service is healthy (2 models available)
✓ NATS Messaging is healthy
✓ Database is healthy (5 tables)
✓ Docker containers are healthy (4/4 running)
✓ System resources are healthy (Disk: 45%, Memory: 60%)
✓ Network connectivity is healthy

🎉 All systems are healthy!
Check completed at ${new Date().toLocaleString()}`
            };
        } else if (command.includes('pull-model')) {
            return {
                success: true,
                output: `📥 HexaRAG Model Download Utility

Starting download of model: llama3.2:3b
Download started. Monitoring progress...
⠋ Downloading... (15s elapsed)
⠙ Downloading... (30s elapsed)
⠹ Downloading... (45s elapsed)
✅ Model 'llama3.2:3b' downloaded successfully!
✓ Model verification passed
🎉 Model 'llama3.2:3b' is ready to use!`
            };
        } else if (command.includes('benchmark')) {
            return {
                success: true,
                output: `📊 HexaRAG Performance Benchmark

🔥 Running warmup (2 requests)...
🗣️  Running conversation benchmark...
Model: deepseek-r1:8b, Requests: 10, Concurrent: 1

📊 Benchmark Results
===================
Total Duration: 25s
Successful Requests: 10
Failed Requests: 0
Success Rate: 100.00%
Throughput: 0.40 req/s

Response Times (ms):
  Min:    180
  Max:    450
  Mean:   285
  Median: 275
  P95:    420
  P99:    445

🎉 Benchmark completed!`
            };
        } else {
            return {
                success: true,
                output: `Command executed: ${command}

✅ Script completed successfully
Output would appear here in a real implementation.
This is a demonstration of the developer dashboard interface.

For actual script execution, the dashboard would:
1. Send the command to the backend API
2. Stream real-time output via WebSocket
3. Display progress and results here
4. Allow export of logs and results`
            };
        }
    }
    
    // Quick Actions
    async runQuickSetup() {
        this.showProgress('Setting up development environment...');
        await this.runScript('dev-setup');
        this.hideProgress();
    }
    
    async runHealthCheck() {
        await this.runScript('health-check');
    }
    
    async runBenchmark() {
        await this.runScript('benchmark');
    }
    
    showResetDbConfirmation() {
        if (confirm('⚠️ This will reset the database. Are you sure?')) {
            this.runScript('reset-db');
        }
    }
    
    // Console Functions
    executeCommand() {
        const input = document.getElementById('consoleInput');
        const command = input.value.trim();
        
        if (!command) return;
        
        // Add command to console
        this.addConsoleEntry('$', command, 'command');
        
        // Execute command (mock implementation)
        this.processCommand(command);
        
        // Clear input
        input.value = '';
    }
    
    async processCommand(command) {
        try {
            // Mock command processing
            await new Promise(resolve => setTimeout(resolve, 500));
            
            if (command.startsWith('make ')) {
                const makeTarget = command.substring(5);
                this.addConsoleEntry('ℹ', `Executing make target: ${makeTarget}`, 'info');
                
                // Simulate make command execution
                setTimeout(() => {
                    this.addConsoleEntry('✓', `Make target '${makeTarget}' completed successfully`, 'success');
                }, 1000);
            } else if (command === 'help') {
                this.addConsoleEntry('ℹ', 'Available commands: make health, make test-chat, make pull-model, make benchmark', 'info');
            } else {
                this.addConsoleEntry('ℹ', `Command executed: ${command}`, 'info');
                this.addConsoleEntry('✓', 'Command completed', 'success');
            }
        } catch (error) {
            this.addConsoleEntry('✗', `Error: ${error.message}`, 'error');
        }
    }
    
    addConsoleEntry(prompt, text, type = 'normal') {
        const output = document.getElementById('consoleOutput');
        const line = document.createElement('div');
        line.className = `console-line ${type}`;
        line.innerHTML = `
            <span class="console-prompt">${prompt}</span>
            <span class="console-text">${text}</span>
        `;
        output.appendChild(line);
        output.scrollTop = output.scrollHeight;
    }
    
    clearConsole() {
        const output = document.getElementById('consoleOutput');
        output.innerHTML = `
            <div class="console-line welcome">
                <span class="console-prompt">$</span>
                <span class="console-text">Console cleared</span>
            </div>
        `;
    }
    
    // Logging Functions
    addLogEntry(level, message) {
        const logs = document.getElementById('logsContent');
        const entry = document.createElement('div');
        entry.className = `log-entry ${level}`;
        entry.innerHTML = `
            <span class="log-timestamp">[${new Date().toLocaleTimeString()}]</span>
            <span class="log-level ${level}">${level.toUpperCase()}</span>
            <span class="log-message">${message}</span>
        `;
        logs.appendChild(entry);
        logs.scrollTop = logs.scrollHeight;
        
        // Apply current filters
        this.filterLogs();
    }
    
    filterLogs() {
        const levelFilter = document.getElementById('logLevel').value;
        const searchFilter = document.getElementById('logSearch').value.toLowerCase();
        const entries = document.querySelectorAll('.log-entry');
        
        entries.forEach(entry => {
            const level = entry.querySelector('.log-level').textContent.toLowerCase();
            const message = entry.querySelector('.log-message').textContent.toLowerCase();
            
            const levelMatch = levelFilter === 'all' || level === levelFilter;
            const searchMatch = !searchFilter || message.includes(searchFilter);
            
            entry.style.display = levelMatch && searchMatch ? 'flex' : 'none';
        });
    }
    
    clearLogs() {
        document.getElementById('logsContent').innerHTML = '';
    }
    
    loadRecentLogs() {
        // Mock recent logs
        const mockLogs = [
            { level: 'info', message: 'Dashboard initialized successfully' },
            { level: 'info', message: 'WebSocket connection established' },
            { level: 'info', message: 'Health check completed' },
            { level: 'warn', message: 'Model download taking longer than expected' },
            { level: 'info', message: 'Performance metrics updated' }
        ];
        
        mockLogs.forEach(log => {
            this.addLogEntry(log.level, log.message);
        });
    }
    
    // Modal Functions
    showScriptModal(title, content) {
        document.getElementById('scriptModalTitle').textContent = title;
        document.getElementById('scriptOutput').textContent = content;
        document.getElementById('scriptModal').classList.add('show');
    }
    
    updateScriptModal(content, success = true) {
        const output = document.getElementById('scriptOutput');
        output.textContent = content;
        output.style.color = success ? '#f0f6fc' : '#da3633';
    }
    
    closeModal() {
        document.querySelectorAll('.modal').forEach(modal => {
            modal.classList.remove('show');
        });
    }
    
    // Progress Functions
    showProgress(text) {
        const progress = document.getElementById('setupProgress');
        const progressText = progress.querySelector('.progress-text');
        const progressFill = progress.querySelector('.progress-fill');
        
        progressText.textContent = text;
        progressFill.style.width = '0%';
        progress.style.display = 'block';
        
        // Animate progress
        let width = 0;
        const interval = setInterval(() => {
            width += Math.random() * 10;
            if (width > 90) width = 90;
            progressFill.style.width = `${width}%`;
        }, 200);
        
        this.progressInterval = interval;
    }
    
    hideProgress() {
        const progress = document.getElementById('setupProgress');
        const progressFill = progress.querySelector('.progress-fill');
        
        if (this.progressInterval) {
            clearInterval(this.progressInterval);
        }
        
        progressFill.style.width = '100%';
        setTimeout(() => {
            progress.style.display = 'none';
        }, 1000);
    }
    
    // Model Functions
    async switchToModel(modelName) {
        try {
            const response = await fetch(`${this.apiBase}/models/current`, {
                method: 'PUT',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ model: modelName })
            });
            
            if (response.ok) {
                this.addLogEntry('info', `Switched to model: ${modelName}`);
                this.loadModels();
            } else {
                throw new Error(`Failed to switch model: ${response.statusText}`);
            }
        } catch (error) {
            this.showError(`Failed to switch model: ${error.message}`);
        }
    }
    
    async deleteModel(modelName) {
        if (!confirm(`Delete model '${modelName}'? This cannot be undone.`)) {
            return;
        }
        
        try {
            const response = await fetch(`${this.apiBase}/models/${encodeURIComponent(modelName)}`, {
                method: 'DELETE'
            });
            
            if (response.ok) {
                this.addLogEntry('info', `Deleted model: ${modelName}`);
                this.loadModels();
            } else {
                throw new Error(`Failed to delete model: ${response.statusText}`);
            }
        } catch (error) {
            this.showError(`Failed to delete model: ${error.message}`);
        }
    }
    
    showDownloadModelDialog() {
        const modelName = prompt('Enter model name to download:');
        if (modelName) {
            document.getElementById('downloadModel').value = modelName;
            this.runScript('pull-model');
        }
    }
    
    // Data Polling
    startMetricsPolling() {
        this.refreshInterval = setInterval(() => {
            this.loadSystemHealth();
            this.loadPerformanceMetrics();
            this.loadConnections();
        }, 30000); // Refresh every 30 seconds
    }
    
    refreshAllData() {
        console.log('🔄 Refreshing all data');
        this.addLogEntry('info', 'Refreshing dashboard data');
        this.loadInitialData();
    }
    
    // Utility Functions
    formatUptime(seconds) {
        const hours = Math.floor(seconds / 3600);
        const minutes = Math.floor((seconds % 3600) / 60);
        return `${hours}h ${minutes}m`;
    }
    
    formatBytes(bytes) {
        if (bytes === 0) return '0 B';
        const k = 1024;
        const sizes = ['B', 'KB', 'MB', 'GB'];
        const i = Math.floor(Math.log(bytes) / Math.log(k));
        return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + ' ' + sizes[i];
    }
    
    formatDate(dateString) {
        if (!dateString) return '--';
        return new Date(dateString).toLocaleDateString();
    }
    
    generateMockPerformanceData() {
        return Array.from({ length: 20 }, () => Math.random() * 500 + 100);
    }
    
    showError(message) {
        this.addLogEntry('error', message);
        alert(message); // Simple error display
    }
    
    // Script Help Functions
    showScriptHelp(scriptName) {
        const helpContent = this.getScriptHelp(scriptName);
        this.showScriptModal(`${scriptName} - Help`, helpContent);
    }
    
    getScriptHelp(scriptName) {
        const helpTexts = {
            'test-chat': `Interactive Chat Testing Tool

Usage: test-chat.sh [OPTIONS]

This tool provides an interactive command-line interface for testing conversations with the HexaRAG API.

Options:
- Model: Select which AI model to use for testing
- Extended Knowledge: Enable extended knowledge retrieval
- Verbose Mode: Show detailed debugging information

Features:
- Real-time conversation testing
- Model switching during chat
- Conversation history saving
- Command help system`,

            'pull-model': `Model Download Utility

Usage: pull-model.sh [OPTIONS] [MODEL_NAME]

Download and manage AI models from the Ollama registry.

Popular Models:
- llama3.2:1b (1.3GB) - Lightweight
- llama3.2:3b (2.0GB) - Balanced
- deepseek-r1:1.5b (1.7GB) - Fast reasoning
- deepseek-r1:8b (8.9GB) - Advanced reasoning

Features:
- Progress tracking
- Model verification
- Batch downloads
- Available model listing`,

            'health-check': `System Health Verification

Usage: health-check.sh [OPTIONS]

Comprehensive health check for all HexaRAG components.

Checks:
- API Server response time
- Ollama service and models
- NATS messaging system
- Database accessibility
- Docker containers
- System resources
- Network connectivity

Options:
- Detailed: Show component details
- Verbose: Show debugging info
- Timeout: Set request timeout`,

            'benchmark': `Performance Testing Tool

Usage: benchmark.sh [OPTIONS]

Test system performance and generate metrics.

Test Types:
- Conversation: Full conversation flow
- Load: Multiple concurrent users
- Stress: Find system limits

Metrics:
- Response times (min, max, avg, P95, P99)
- Throughput (requests per second)
- Success/failure rates
- Detailed performance statistics`
        };
        
        return helpTexts[scriptName] || 'No help available for this script.';
    }
    
    // Cleanup
    destroy() {
        if (this.refreshInterval) {
            clearInterval(this.refreshInterval);
        }
        
        if (this.wsConnection) {
            this.wsConnection.close();
        }
        
        Object.values(this.charts).forEach(chart => {
            chart.destroy();
        });
    }
}

// Initialize dashboard when DOM is loaded
document.addEventListener('DOMContentLoaded', () => {
    window.dashboard = new DeveloperDashboard();
});

// Handle page unload
window.addEventListener('beforeunload', () => {
    if (window.dashboard) {
        window.dashboard.destroy();
    }
});