/**
 * HexaRAG Chat Application
 * Handles WebSocket connections, API calls, and chat interface interactions
 */

/**
 * Model Manager class for handling model operations
 */
class ModelManager {
    constructor(apiBase, wsConnection) {
        this.api = apiBase;
        this.ws = wsConnection;
        this.models = new Map();
        this.downloadQueue = new Map();
        this.currentModel = null;
    }
    
    async loadModels() {
        try {
            const response = await fetch(`${this.api}/models`);
            const data = await response.json();
            
            // Update models map
            this.models.clear();
            data.models.forEach(model => {
                this.models.set(model.id, model);
            });
            
            return data.models;
        } catch (error) {
            console.error('Failed to load models:', error);
            throw error;
        }
    }
    
    async pullModel(modelName) {
        try {
            const response = await fetch(`${this.api}/models/pull`, {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ model: modelName })
            });
            
            if (!response.ok) {
                throw new Error(`Failed to start model pull: ${response.statusText}`);
            }
            
            // Add to download queue
            this.downloadQueue.set(modelName, {
                model: modelName,
                status: 'starting',
                progress: 0,
                startTime: Date.now()
            });
            
            return await response.json();
        } catch (error) {
            console.error('Failed to pull model:', error);
            throw error;
        }
    }
    
    async deleteModel(modelName) {
        try {
            const response = await fetch(`${this.api}/models/${encodeURIComponent(modelName)}`, {
                method: 'DELETE'
            });
            
            if (!response.ok) {
                throw new Error(`Failed to delete model: ${response.statusText}`);
            }
            
            // Remove from local models map
            this.models.delete(modelName);
            
            return await response.json();
        } catch (error) {
            console.error('Failed to delete model:', error);
            throw error;
        }
    }
    
    async switchModel(modelName, conversationId = null) {
        try {
            const response = await fetch(`${this.api}/models/current`, {
                method: 'PUT',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ 
                    model: modelName,
                    conversation_id: conversationId 
                })
            });
            
            if (!response.ok) {
                throw new Error(`Failed to switch model: ${response.statusText}`);
            }
            
            this.currentModel = modelName;
            return await response.json();
        } catch (error) {
            console.error('Failed to switch model:', error);
            throw error;
        }
    }
    
    async getModelInfo(modelName) {
        try {
            const response = await fetch(`${this.api}/models/${encodeURIComponent(modelName)}`);
            
            if (!response.ok) {
                throw new Error(`Failed to get model info: ${response.statusText}`);
            }
            
            return await response.json();
        } catch (error) {
            console.error('Failed to get model info:', error);
            throw error;
        }
    }
    
    async getModelStatus() {
        try {
            const response = await fetch(`${this.api}/models/status`);
            
            if (!response.ok) {
                throw new Error(`Failed to get model status: ${response.statusText}`);
            }
            
            return await response.json();
        } catch (error) {
            console.error('Failed to get model status:', error);
            throw error;
        }
    }
    
    handleWebSocketMessage(message) {
        switch(message.type) {
            case 'model_pull_progress':
                this.updateProgress(message.model, message.percent, message.status);
                break;
            case 'model_pull_complete':
                this.onModelReady(message.model);
                break;
            case 'model_pull_error':
                this.onModelError(message.model, message.error);
                break;
            case 'model_switched':
                this.onModelSwitched(message.model, message.conversation_id);
                break;
            case 'model_deleted':
                this.onModelDeleted(message.model);
                break;
        }
    }
    
    updateProgress(modelName, percent, status) {
        if (this.downloadQueue.has(modelName)) {
            const download = this.downloadQueue.get(modelName);
            download.progress = percent;
            download.status = status;
            this.downloadQueue.set(modelName, download);
        }
        
        // Trigger UI update
        this.onProgressUpdate?.(modelName, percent, status);
    }
    
    onModelReady(modelName) {
        // Remove from download queue
        this.downloadQueue.delete(modelName);
        
        // Trigger UI update
        this.onModelComplete?.(modelName);
    }
    
    onModelError(modelName, error) {
        // Update download queue with error
        if (this.downloadQueue.has(modelName)) {
            const download = this.downloadQueue.get(modelName);
            download.status = 'error';
            download.error = error;
            this.downloadQueue.set(modelName, download);
        }
        
        // Trigger UI update
        this.onModelError?.(modelName, error);
    }
    
    onModelSwitched(modelName, conversationId) {
        this.currentModel = modelName;
        this.onSwitchComplete?.(modelName, conversationId);
    }
    
    onModelDeleted(modelName) {
        this.models.delete(modelName);
        this.onDeleteComplete?.(modelName);
    }
}

class HexaRAGChat {
    constructor() {
        this.apiBase = '/api/v1';
        this.wsUrl = `ws://${window.location.host}/ws`;
        this.ws = null;
        this.isConnected = false;
        this.currentConversationId = null;
        this.conversations = new Map();
        this.isTyping = false;
        
        // Initialize model manager
        this.modelManager = new ModelManager(this.apiBase, this.ws);
        this.setupModelManagerCallbacks();
        
        this.initializeElements();
        this.initializeEventListeners();
        this.connectWebSocket();
        this.loadConversations();
        this.loadModels();
    }

    initializeElements() {
        // Main elements
        this.messageInput = document.getElementById('messageInput');
        this.sendBtn = document.getElementById('sendBtn');
        this.messagesContainer = document.getElementById('messagesContainer');
        this.conversationsList = document.getElementById('conversationsList');
        this.connectionStatus = document.getElementById('connectionStatus');
        this.loadingOverlay = document.getElementById('loadingOverlay');
        
        // Header elements
        this.chatTitle = document.getElementById('chatTitle');
        this.chatInfo = document.getElementById('chatInfo');
        this.modelSelector = document.getElementById('modelSelector');
        this.newChatBtn = document.getElementById('newChatBtn');
        this.refreshBtn = document.getElementById('refreshConversations');
        this.manageModelsBtn = document.getElementById('manageModelsBtn');
        
        // Modal elements
        this.modelModal = document.getElementById('modelModal');
        this.closeModalBtn = document.getElementById('closeModalBtn');
        this.modelSearch = document.getElementById('modelSearch');
        this.pullSearchedModel = document.getElementById('pullSearchedModel');
        this.modelGrid = document.getElementById('modelGrid');
        this.downloadQueue = document.getElementById('downloadQueue');
        this.downloadSection = document.getElementById('downloadSection');
        this.runningModels = document.getElementById('runningModels');
        
        // Confirmation dialog
        this.confirmDialog = document.getElementById('confirmDialog');
        this.confirmTitle = document.getElementById('confirmTitle');
        this.confirmMessage = document.getElementById('confirmMessage');
        this.confirmOk = document.getElementById('confirmOk');
        this.confirmCancel = document.getElementById('confirmCancel');
        
        // Options
        this.extendedKnowledge = document.getElementById('extendedKnowledge');
    }

    initializeEventListeners() {
        // Send message
        this.sendBtn.addEventListener('click', () => this.sendMessage());
        
        // Enter to send, Shift+Enter for new line
        this.messageInput.addEventListener('keydown', (e) => {
            if (e.key === 'Enter' && !e.shiftKey) {
                e.preventDefault();
                this.sendMessage();
            }
        });

        // Auto-resize textarea
        this.messageInput.addEventListener('input', () => {
            this.autoResizeTextarea();
        });

        // New chat button
        this.newChatBtn.addEventListener('click', () => this.createNewConversation());
        
        // Refresh conversations
        this.refreshBtn.addEventListener('click', () => this.loadConversations());
        
        // Model selector
        this.modelSelector.addEventListener('change', () => {
            this.switchModel(this.modelSelector.value);
        });
        
        // Model management
        this.manageModelsBtn.addEventListener('click', () => this.openModelModal());
        this.closeModalBtn.addEventListener('click', () => this.closeModelModal());
        this.pullSearchedModel.addEventListener('click', () => this.pullModelFromSearch());
        
        // Confirmation dialog
        this.confirmCancel.addEventListener('click', () => this.closeConfirmDialog());
        
        // Modal close on background click
        this.modelModal.addEventListener('click', (e) => {
            if (e.target === this.modelModal) {
                this.closeModelModal();
            }
        });
        
        this.confirmDialog.addEventListener('click', (e) => {
            if (e.target === this.confirmDialog) {
                this.closeConfirmDialog();
            }
        });
    }

    // WebSocket Connection Management
    connectWebSocket() {
        try {
            this.updateConnectionStatus('connecting', 'Connecting...');
            this.ws = new WebSocket(this.wsUrl);
            
            this.ws.onopen = () => {
                console.log('WebSocket connected');
                this.isConnected = true;
                this.updateConnectionStatus('connected', 'Connected');
                this.enableInputs();
            };
            
            this.ws.onclose = () => {
                console.log('WebSocket disconnected');
                this.isConnected = false;
                this.updateConnectionStatus('disconnected', 'Disconnected');
                this.disableInputs();
                
                // Attempt to reconnect after 3 seconds
                setTimeout(() => this.connectWebSocket(), 3000);
            };
            
            this.ws.onerror = (error) => {
                console.error('WebSocket error:', error);
                this.updateConnectionStatus('error', 'Connection error');
            };
            
            this.ws.onmessage = (event) => {
                this.handleWebSocketMessage(event);
            };
            
        } catch (error) {
            console.error('Failed to connect WebSocket:', error);
            this.updateConnectionStatus('error', 'Failed to connect');
        }
    }

    handleWebSocketMessage(event) {
        try {
            const data = JSON.parse(event.data);
            console.log('WebSocket message:', data);
            
            // Check if this is a model-related message
            if (data.type?.startsWith('model_')) {
                this.modelManager.handleWebSocketMessage(data);
                return;
            }
            
            switch (data.type) {
                case 'message_start':
                    this.handleMessageStart(data);
                    break;
                case 'message_chunk':
                    this.handleMessageChunk(data);
                    break;
                case 'message_complete':
                    this.handleMessageComplete(data);
                    break;
                case 'response':
                    this.handleResponse(data);
                    break;
                case 'error':
                    this.handleError(data);
                    break;
                case 'status':
                    this.handleStatus(data);
                    break;
                case 'pong':
                    console.log('WebSocket pong received');
                    break;
                default:
                    console.log('Unknown message type:', data.type);
            }
        } catch (error) {
            console.error('Error parsing WebSocket message:', error);
        }
    }

    handleMessageStart(data) {
        this.removeTypingIndicator();
        this.addAssistantMessage('', data.message_id, true); // streaming = true
    }

    handleMessageChunk(data) {
        this.appendToLastMessage(data.content);
    }

    handleMessageComplete(data) {
        this.finalizeLastMessage(data.message_id);
        this.hideLoading();
        this.enableInputs();
    }

    handleResponse(data) {
        // Handle non-streaming response (fallback)
        this.removeTypingIndicator();
        this.addAssistantMessage(data.content || JSON.stringify(data.data), data.message_id, false);
        this.hideLoading();
        this.enableInputs();
    }

    handleStatus(data) {
        console.log('Status update:', data.content);
        if (data.content === 'subscribed') {
            console.log(`Successfully subscribed to conversation ${data.conversation_id}`);
        }
    }

    handleError(data) {
        console.error('WebSocket error:', data.content || data.error);
        this.removeTypingIndicator();
        this.addSystemMessage(`Error: ${data.content || data.error}`);
        this.hideLoading();
        this.enableInputs();
    }

    // API Calls
    async apiCall(endpoint, options = {}) {
        const url = `${this.apiBase}${endpoint}`;
        const config = {
            headers: {
                'Content-Type': 'application/json',
                ...options.headers
            },
            ...options
        };
        
        try {
            const response = await fetch(url, config);
            if (!response.ok) {
                throw new Error(`HTTP ${response.status}: ${response.statusText}`);
            }
            return await response.json();
        } catch (error) {
            console.error(`API call failed: ${endpoint}`, error);
            throw error;
        }
    }

    async loadConversations() {
        try {
            this.conversationsList.innerHTML = '<div class="loading">Loading conversations...</div>';
            
            const response = await this.apiCall('/conversations');
            const conversations = response.conversations || [];
            
            this.conversations.clear();
            conversations.forEach(conv => {
                this.conversations.set(conv.id, conv);
            });
            
            this.renderConversations();
        } catch (error) {
            console.error('Failed to load conversations:', error);
            this.conversationsList.innerHTML = '<div class="loading">Failed to load conversations</div>';
        }
    }

    async loadModels() {
        try {
            const models = await this.modelManager.loadModels();
            this.updateModelSelector(models);
            console.log(`Loaded ${models.length} models`);
        } catch (error) {
            console.error('Failed to load models:', error);
            // Keep default option if API fails
        }
    }
    
    updateModelSelector(models) {
        // Clear existing options
        this.modelSelector.innerHTML = '';
        
        // Add available models
        models.forEach(model => {
            const option = document.createElement('option');
            option.value = model.id;
            option.textContent = model.name;
            option.disabled = !model.available;
            option.title = model.description;
            
            if (model.id === 'deepseek-r1:8b') {
                option.selected = true;
                this.modelManager.currentModel = model.id;
            }
            
            this.modelSelector.appendChild(option);
        });
    }
    
    async switchModel(modelName) {
        try {
            await this.modelManager.switchModel(modelName, this.currentConversationId);
            console.log(`Model switched to: ${modelName}`);
        } catch (error) {
            console.error('Failed to switch model:', error);
            // Revert selector to previous value
            this.modelSelector.value = this.modelManager.currentModel || '';
        }
    }
    
    setupModelManagerCallbacks() {
        // Set up callbacks for model manager events
        this.modelManager.onProgressUpdate = (modelName, percent, status) => {
            this.updateModelProgress(modelName, percent, status);
        };
        
        this.modelManager.onModelComplete = (modelName) => {
            this.onModelDownloadComplete(modelName);
        };
        
        this.modelManager.onModelError = (modelName, error) => {
            this.onModelDownloadError(modelName, error);
        };
        
        this.modelManager.onSwitchComplete = (modelName, conversationId) => {
            this.onModelSwitchComplete(modelName, conversationId);
        };
        
        this.modelManager.onDeleteComplete = (modelName) => {
            this.onModelDeleteComplete(modelName);
        };
    }
    
    updateModelProgress(modelName, percent, status) {
        // Find model progress element and update it
        const progressElement = document.querySelector(`[data-model="${modelName}"] .progress-fill`);
        if (progressElement) {
            progressElement.style.width = `${percent}%`;
        }
        
        const statusElement = document.querySelector(`[data-model="${modelName}"] .progress-text`);
        if (statusElement) {
            statusElement.textContent = `${Math.round(percent)}% - ${status}`;
        }
        
        console.log(`Model ${modelName}: ${percent}% - ${status}`);
    }
    
    onModelDownloadComplete(modelName) {
        console.log(`Model download complete: ${modelName}`);
        
        // Remove from download queue UI
        const downloadElement = document.querySelector(`[data-model="${modelName}"]`);
        if (downloadElement && downloadElement.closest('.download-item')) {
            downloadElement.closest('.download-item').remove();
        }
        
        // Refresh model list
        this.loadModels();
        
        // Show success message
        this.addSystemMessage(`Model ${modelName} downloaded successfully`);
    }
    
    onModelDownloadError(modelName, error) {
        console.error(`Model download error for ${modelName}:`, error);
        
        // Update UI to show error
        const statusElement = document.querySelector(`[data-model="${modelName}"] .progress-text`);
        if (statusElement) {
            statusElement.textContent = `Error: ${error}`;
            statusElement.style.color = 'var(--error-color)';
        }
        
        // Show error message
        this.addSystemMessage(`Failed to download ${modelName}: ${error}`);
    }
    
    onModelSwitchComplete(modelName, conversationId) {
        console.log(`Model switched to ${modelName} for conversation ${conversationId || 'global'}`);
        
        // Update model selector
        if (this.modelSelector.value !== modelName) {
            this.modelSelector.value = modelName;
        }
        
        // Show success message
        if (conversationId) {
            this.addSystemMessage(`Model switched to ${modelName} for this conversation`);
        }
    }
    
    onModelDeleteComplete(modelName) {
        console.log(`Model deleted: ${modelName}`);
        
        // Remove from model selector if it was the selected model
        if (this.modelSelector.value === modelName) {
            this.modelSelector.value = '';
        }
        
        // Refresh model list
        this.loadModels();
        
        // Show success message
        this.addSystemMessage(`Model ${modelName} deleted successfully`);
    }

    async createNewConversation() {
        try {
            this.showLoading('Creating new conversation...');
            
            const response = await this.apiCall('/conversations', {
                method: 'POST',
                body: JSON.stringify({
                    title: 'New Chat',
                    system_prompt_id: 'default'
                })
            });
            
            this.conversations.set(response.id, response);
            this.selectConversation(response.id);
            this.renderConversations();
            this.hideLoading();
            
        } catch (error) {
            console.error('Failed to create conversation:', error);
            this.hideLoading();
            alert('Failed to create new conversation');
        }
    }

    async loadMessages(conversationId) {
        try {
            const response = await this.apiCall(`/conversations/${conversationId}/messages`);
            return response.messages || [];
        } catch (error) {
            console.error('Failed to load messages:', error);
            return [];
        }
    }

    async sendMessage() {
        const content = this.messageInput.value.trim();
        if (!content || !this.currentConversationId || !this.isConnected) {
            return;
        }

        try {
            this.disableInputs();
            this.showLoading('Sending message...');
            
            // Add user message to UI
            this.addUserMessage(content);
            this.messageInput.value = '';
            this.autoResizeTextarea();
            
            // Add typing indicator
            this.addTypingIndicator();
            
            // Send message via API
            const response = await this.apiCall(`/conversations/${this.currentConversationId}/messages`, {
                method: 'POST',
                body: JSON.stringify({
                    content: content,
                    use_extended_knowledge: this.extendedKnowledge.checked
                })
            });
            
            console.log('Message sent:', response);
            this.hideLoading();
            
        } catch (error) {
            console.error('Failed to send message:', error);
            this.removeTypingIndicator();
            this.addSystemMessage('Failed to send message. Please try again.');
            this.hideLoading();
            this.enableInputs();
        }
    }

    // UI Management
    renderConversations() {
        if (this.conversations.size === 0) {
            this.conversationsList.innerHTML = `
                <div class="loading">
                    No conversations yet.<br>
                    <button class="btn btn-primary" onclick="hexarag.createNewConversation()">
                        Start New Chat
                    </button>
                </div>
            `;
            return;
        }

        const conversationsArray = Array.from(this.conversations.values())
            .sort((a, b) => new Date(b.updated_at) - new Date(a.updated_at));

        this.conversationsList.innerHTML = conversationsArray.map(conv => `
            <div class="conversation-item ${conv.id === this.currentConversationId ? 'active' : ''}" 
                 onclick="hexarag.selectConversation('${conv.id}')">
                <div class="conversation-title">${this.escapeHtml(conv.title)}</div>
                <div class="conversation-meta">
                    <span>${conv.message_ids.length} messages</span>
                    <span>${this.formatTime(conv.updated_at)}</span>
                </div>
            </div>
        `).join('');
    }

    async selectConversation(conversationId) {
        if (this.currentConversationId === conversationId) return;
        
        this.currentConversationId = conversationId;
        const conversation = this.conversations.get(conversationId);
        
        if (!conversation) return;
        
        // Subscribe to conversation via WebSocket
        if (this.ws && this.isConnected) {
            const subscribeMsg = {
                type: 'subscribe',
                conversation_id: conversationId,
                timestamp: new Date().toISOString()
            };
            this.ws.send(JSON.stringify(subscribeMsg));
        }
        
        // Update UI
        this.chatTitle.textContent = conversation.title;
        this.chatInfo.textContent = `${conversation.message_ids.length} messages`;
        this.renderConversations();
        
        // Load and display messages
        this.clearMessages();
        this.showLoading('Loading messages...');
        
        try {
            const messages = await this.loadMessages(conversationId);
            messages.forEach(message => {
                if (message.role === 'user') {
                    this.addUserMessage(message.content, message.created_at);
                } else if (message.role === 'assistant') {
                    this.addAssistantMessage(message.content, message.id, false, message.created_at);
                }
            });
            this.scrollToBottom();
        } catch (error) {
            console.error('Failed to load messages:', error);
            this.addSystemMessage('Failed to load messages');
        }
        
        this.hideLoading();
        this.enableInputs();
    }

    addUserMessage(content, timestamp = null) {
        const time = timestamp || new Date().toISOString();
        const messageHtml = `
            <div class="message user">
                <div class="message-avatar">👤</div>
                <div class="message-content">
                    <div class="message-bubble">${this.escapeHtml(content)}</div>
                    <div class="message-time">${this.formatTime(time)}</div>
                </div>
            </div>
        `;
        this.messagesContainer.insertAdjacentHTML('beforeend', messageHtml);
        this.scrollToBottom();
    }

    addAssistantMessage(content, messageId = null, streaming = false, timestamp = null) {
        const time = timestamp || new Date().toISOString();
        const messageHtml = `
            <div class="message assistant" data-message-id="${messageId || 'streaming'}">
                <div class="message-avatar">🤖</div>
                <div class="message-content">
                    <div class="message-bubble">${streaming ? '' : this.escapeHtml(content)}</div>
                    <div class="message-time">${this.formatTime(time)}</div>
                </div>
            </div>
        `;
        this.messagesContainer.insertAdjacentHTML('beforeend', messageHtml);
        this.scrollToBottom();
    }

    appendToLastMessage(content) {
        const lastMessage = this.messagesContainer.querySelector('.message.assistant:last-child .message-bubble');
        if (lastMessage) {
            lastMessage.textContent += content;
            this.scrollToBottom();
        }
    }

    finalizeLastMessage(messageId) {
        const lastMessage = this.messagesContainer.querySelector('.message.assistant:last-child');
        if (lastMessage && messageId) {
            lastMessage.setAttribute('data-message-id', messageId);
        }
    }

    addTypingIndicator() {
        if (this.isTyping) return;
        
        this.isTyping = true;
        const typingHtml = `
            <div class="typing-indicator" id="typingIndicator">
                <div class="message-avatar">🤖</div>
                <div class="typing-text">
                    Thinking
                    <div class="typing-dots">
                        <div class="typing-dot"></div>
                        <div class="typing-dot"></div>
                        <div class="typing-dot"></div>
                    </div>
                </div>
            </div>
        `;
        this.messagesContainer.insertAdjacentHTML('beforeend', typingHtml);
        this.scrollToBottom();
    }

    removeTypingIndicator() {
        const indicator = document.getElementById('typingIndicator');
        if (indicator) {
            indicator.remove();
            this.isTyping = false;
        }
    }

    addSystemMessage(content) {
        const messageHtml = `
            <div class="message system">
                <div class="message-content">
                    <div class="message-bubble" style="background: var(--warning-color); color: white;">
                        ${this.escapeHtml(content)}
                    </div>
                </div>
            </div>
        `;
        this.messagesContainer.insertAdjacentHTML('beforeend', messageHtml);
        this.scrollToBottom();
    }

    clearMessages() {
        this.messagesContainer.innerHTML = '';
    }

    // UI State Management
    updateConnectionStatus(state, text) {
        this.connectionStatus.className = `status ${state}`;
        this.connectionStatus.querySelector('.status-text').textContent = text;
    }

    enableInputs() {
        if (this.currentConversationId && this.isConnected) {
            this.messageInput.disabled = false;
            this.sendBtn.disabled = false;
            this.messageInput.placeholder = 'Type your message...';
        }
    }

    disableInputs() {
        this.messageInput.disabled = true;
        this.sendBtn.disabled = true;
        this.messageInput.placeholder = 'Please wait...';
    }

    showLoading(text = 'Loading...') {
        this.loadingOverlay.classList.add('show');
        this.loadingOverlay.querySelector('.loading-text').textContent = text;
    }

    hideLoading() {
        this.loadingOverlay.classList.remove('show');
    }

    autoResizeTextarea() {
        this.messageInput.style.height = 'auto';
        this.messageInput.style.height = Math.min(this.messageInput.scrollHeight, 120) + 'px';
    }

    scrollToBottom() {
        setTimeout(() => {
            this.messagesContainer.scrollTop = this.messagesContainer.scrollHeight;
        }, 100);
    }

    // Utility Functions
    escapeHtml(text) {
        const div = document.createElement('div');
        div.textContent = text;
        return div.innerHTML;
    }

    formatTime(timestamp) {
        const date = new Date(timestamp);
        const now = new Date();
        const diffMs = now - date;
        const diffMins = Math.floor(diffMs / 60000);
        const diffHours = Math.floor(diffMins / 60);
        const diffDays = Math.floor(diffHours / 24);

        if (diffMins < 1) return 'Just now';
        if (diffMins < 60) return `${diffMins}m ago`;
        if (diffHours < 24) return `${diffHours}h ago`;
        if (diffDays < 7) return `${diffDays}d ago`;
        
        return date.toLocaleDateString();
    }
    
    // Model Management UI Methods
    async openModelModal() {
        this.modelModal.style.display = 'flex';
        await this.loadModalContent();
    }
    
    closeModelModal() {
        this.modelModal.style.display = 'none';
    }
    
    async loadModalContent() {
        // Load available models
        try {
            const models = await this.modelManager.loadModels();
            this.renderModelGrid(models);
        } catch (error) {
            this.modelGrid.innerHTML = '<div class="error">Failed to load models</div>';
        }
        
        // Load running models
        try {
            const status = await this.modelManager.getModelStatus();
            this.renderRunningModels(status.running_models || []);
        } catch (error) {
            this.runningModels.innerHTML = '<div class="error">Failed to load running models</div>';
        }
        
        // Update download queue
        this.updateDownloadQueueUI();
    }
    
    renderModelGrid(models) {
        if (models.length === 0) {
            this.modelGrid.innerHTML = '<div class="empty">No models available</div>';
            return;
        }
        
        this.modelGrid.innerHTML = models.map(model => `
            <div class="model-card" data-model="${model.id}">
                <div class="model-header">
                    <h4>${this.escapeHtml(model.name)}</h4>
                    <div class="model-status ${model.available ? 'available' : 'unavailable'}">
                        ${model.available ? '✓ Available' : '⚠ Offline'}
                    </div>
                </div>
                <div class="model-info">
                    <div class="model-detail">
                        <span class="label">Family:</span>
                        <span class="value">${this.escapeHtml(model.family || 'Unknown')}</span>
                    </div>
                    <div class="model-detail">
                        <span class="label">Parameters:</span>
                        <span class="value">${this.escapeHtml(model.parameters || 'Unknown')}</span>
                    </div>
                    ${model.size ? `
                    <div class="model-detail">
                        <span class="label">Size:</span>
                        <span class="value">${this.formatFileSize(model.size)}</span>
                    </div>
                    ` : ''}
                </div>
                <div class="model-description">
                    ${this.escapeHtml(model.description || '')}
                </div>
                <div class="model-actions">
                    ${model.available ? `
                        <button class="btn btn-primary btn-sm" onclick="hexarag.useModel('${model.id}')">
                            Use Model
                        </button>
                        <button class="btn btn-danger btn-sm" onclick="hexarag.confirmDeleteModel('${model.id}')">
                            Delete
                        </button>
                    ` : `
                        <button class="btn btn-secondary btn-sm" disabled>
                            Unavailable
                        </button>
                    `}
                </div>
            </div>
        `).join('');
    }
    
    renderRunningModels(runningModels) {
        if (runningModels.length === 0) {
            this.runningModels.innerHTML = '<div class="empty">No models currently running</div>';
            return;
        }
        
        this.runningModels.innerHTML = runningModels.map(model => `
            <div class="running-model">
                <div class="model-name">${this.escapeHtml(model.name)}</div>
                <div class="model-stats">
                    <span>Size: ${this.formatFileSize(model.size_vram)}</span>
                    <span>Until: ${this.formatTime(model.expires_at)}</span>
                </div>
            </div>
        `).join('');
    }
    
    async pullModelFromSearch() {
        const modelName = this.modelSearch.value.trim();
        if (!modelName) {
            alert('Please enter a model name');
            return;
        }
        
        try {
            await this.modelManager.pullModel(modelName);
            this.addDownloadToQueue(modelName);
            this.modelSearch.value = '';
            console.log(`Started pulling model: ${modelName}`);
        } catch (error) {
            console.error('Failed to start model pull:', error);
            alert(`Failed to start downloading ${modelName}: ${error.message}`);
        }
    }
    
    async useModel(modelId) {
        try {
            await this.switchModel(modelId);
            this.closeModelModal();
        } catch (error) {
            console.error('Failed to use model:', error);
            alert(`Failed to switch to model ${modelId}: ${error.message}`);
        }
    }
    
    confirmDeleteModel(modelId) {
        this.confirmTitle.textContent = 'Delete Model';
        this.confirmMessage.textContent = `Are you sure you want to delete ${modelId}? This action cannot be undone.`;
        this.confirmOk.onclick = () => this.deleteModel(modelId);
        this.confirmDialog.style.display = 'flex';
    }
    
    async deleteModel(modelId) {
        try {
            await this.modelManager.deleteModel(modelId);
            this.closeConfirmDialog();
            this.loadModalContent(); // Refresh modal content
            console.log(`Deleted model: ${modelId}`);
        } catch (error) {
            console.error('Failed to delete model:', error);
            alert(`Failed to delete model ${modelId}: ${error.message}`);
        }
    }
    
    closeConfirmDialog() {
        this.confirmDialog.style.display = 'none';
        this.confirmOk.onclick = null;
    }
    
    addDownloadToQueue(modelName) {
        // Show download section
        this.downloadSection.style.display = 'block';
        
        // Add download item
        const downloadItem = document.createElement('div');
        downloadItem.className = 'download-item';
        downloadItem.setAttribute('data-model', modelName);
        downloadItem.innerHTML = `
            <div class="download-info">
                <span class="model-name">${this.escapeHtml(modelName)}</span>
                <span class="download-status">Starting...</span>
            </div>
            <div class="progress-bar">
                <div class="progress-fill" style="width: 0%"></div>
            </div>
            <div class="progress-text">0% - Initializing</div>
        `;
        
        this.downloadQueue.appendChild(downloadItem);
    }
    
    updateDownloadQueueUI() {
        const downloads = Array.from(this.modelManager.downloadQueue.values());
        
        if (downloads.length === 0) {
            this.downloadSection.style.display = 'none';
            return;
        }
        
        this.downloadSection.style.display = 'block';
        this.downloadQueue.innerHTML = downloads.map(download => `
            <div class="download-item" data-model="${download.model}">
                <div class="download-info">
                    <span class="model-name">${this.escapeHtml(download.model)}</span>
                    <span class="download-status">${download.status}</span>
                </div>
                <div class="progress-bar">
                    <div class="progress-fill" style="width: ${download.progress}%"></div>
                </div>
                <div class="progress-text">${Math.round(download.progress)}% - ${download.status}</div>
                ${download.error ? `<div class="download-error">Error: ${this.escapeHtml(download.error)}</div>` : ''}
            </div>
        `).join('');
    }
    
    formatFileSize(bytes) {
        if (!bytes) return 'Unknown';
        
        const units = ['B', 'KB', 'MB', 'GB', 'TB'];
        let size = bytes;
        let unitIndex = 0;
        
        while (size >= 1024 && unitIndex < units.length - 1) {
            size /= 1024;
            unitIndex++;
        }
        
        return `${size.toFixed(1)} ${units[unitIndex]}`;
    }
}

// Initialize the application
let hexarag;
document.addEventListener('DOMContentLoaded', () => {
    hexarag = new HexaRAGChat();
});

// Handle page visibility changes to manage WebSocket connection
document.addEventListener('visibilitychange', () => {
    if (document.hidden) {
        // Page is hidden, could pause reconnection attempts
        console.log('Page hidden');
    } else {
        // Page is visible, ensure connection is active
        console.log('Page visible');
        if (hexarag && !hexarag.isConnected) {
            hexarag.connectWebSocket();
        }
    }
});