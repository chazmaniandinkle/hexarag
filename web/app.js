/**
 * HexaRAG Chat Application
 * Handles WebSocket connections, API calls, and chat interface interactions
 */

class HexaRAGChat {
    constructor() {
        this.apiBase = '/api/v1';
        this.wsUrl = `ws://${window.location.host}/ws`;
        this.ws = null;
        this.isConnected = false;
        this.currentConversationId = null;
        this.conversations = new Map();
        this.isTyping = false;
        
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
            // TODO: Update current conversation model
            console.log('Model changed to:', this.modelSelector.value);
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
            const response = await this.apiCall('/models');
            const models = response.models || [];
            
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
                }
                
                this.modelSelector.appendChild(option);
            });
            
            console.log(`Loaded ${models.length} models`);
        } catch (error) {
            console.error('Failed to load models:', error);
            // Keep default option if API fails
        }
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