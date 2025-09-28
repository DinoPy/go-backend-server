const WebSocket = require('ws');

// Test configuration
const WS_URL = 'ws://localhost:8080/ws/taskbar';
const TEST_USER = {
    email: 'test@example.com',
    first_name: 'Test',
    last_name: 'User',
    google_uid: 'test-google-uid-123'
};

// Function to generate a valid UUID v4
function generateUUID() {
    return 'xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx'.replace(/[xy]/g, function(c) {
        const r = Math.random() * 16 | 0;
        const v = c == 'x' ? r : (r & 0x3 | 0x8);
        return v.toString(16);
    });
}

// Test data for task creation with new properties
const TASK_WITH_NEW_PROPERTIES = {
    id: generateUUID(),
    title: 'Test Task with Priority',
    description: 'Test Description',
    created_at: new Date().toISOString(),
    completed_at: new Date().toISOString(),
    duration: '00:00:00',
    category: 'Test',
    tags: ['test', 'priority'],
    toggled_at: 0,
    is_completed: false,
    is_active: false,
    last_modified_at: Date.now(),
    priority: 5,
    due_at: new Date(Date.now() + 24 * 60 * 60 * 1000).toISOString(), // Tomorrow
    show_before_due_time: 300 // 5 minutes
};

// Test data for task creation without new properties (backward compatibility)
const TASK_WITHOUT_NEW_PROPERTIES = {
    id: generateUUID(),
    title: 'Test Task without Priority',
    description: 'Test Description',
    created_at: new Date().toISOString(),
    completed_at: new Date().toISOString(),
    duration: '00:00:00',
    category: 'Test',
    tags: ['test'],
    toggled_at: 0,
    is_completed: false,
    is_active: false,
    last_modified_at: Date.now()
};

// Test data for task editing with new properties
const TASK_EDIT_WITH_NEW_PROPERTIES = {
    id: TASK_WITH_NEW_PROPERTIES.id, // Use the same ID as the created task
    title: 'Updated Test Task with Priority',
    description: 'Updated Description',
    category: 'Updated',
    tags: ['test', 'priority', 'updated'],
    last_modified_at: Date.now(),
    priority: 8,
    due_at: new Date(Date.now() + 48 * 60 * 60 * 1000).toISOString(), // Day after tomorrow
    show_before_due_time: 600 // 10 minutes
};

let ws;
let sessionId;

function connect() {
    return new Promise((resolve, reject) => {
        ws = new WebSocket(WS_URL);
        
        ws.on('open', () => {
            console.log('✅ Connected to WebSocket');
            resolve();
        });
        
        ws.on('message', (data) => {
            const message = JSON.parse(data.toString());
            console.log('📨 Received:', message.event);
            
            if (message.event === 'connected') {
                sessionId = message.data.sid;
                console.log('✅ Session ID:', sessionId);
            }
        });
        
        ws.on('error', (error) => {
            console.error('❌ WebSocket error:', error);
            reject(error);
        });
    });
}

function sendMessage(event, data) {
    const message = {
        event: event,
        data: data
    };
    
    console.log('📤 Sending:', event);
    ws.send(JSON.stringify(message));
}

async function testTaskCreationWithNewProperties() {
    console.log('\n🧪 Testing task creation with new properties...');
    sendMessage('task_create', TASK_WITH_NEW_PROPERTIES);
    
    // Wait for response
    await new Promise(resolve => setTimeout(resolve, 1000));
}

async function testTaskCreationWithoutNewProperties() {
    console.log('\n🧪 Testing task creation without new properties (backward compatibility)...');
    sendMessage('task_create', TASK_WITHOUT_NEW_PROPERTIES);
    
    // Wait for response
    await new Promise(resolve => setTimeout(resolve, 1000));
}

async function testTaskEditingWithNewProperties() {
    console.log('\n🧪 Testing task editing with new properties...');
    sendMessage('task_edit', TASK_EDIT_WITH_NEW_PROPERTIES);
    
    // Wait for response
    await new Promise(resolve => setTimeout(resolve, 1000));
}

async function runTests() {
    try {
        console.log('🚀 Starting WebSocket event tests...');
        
        // Connect and authenticate
        await connect();
        sendMessage('connect', TEST_USER);
        
        // Wait for connection to be established
        await new Promise(resolve => setTimeout(resolve, 2000));
        
        // Run tests
        await testTaskCreationWithNewProperties();
        await testTaskCreationWithoutNewProperties();
        await testTaskEditingWithNewProperties();
        
        console.log('\n✅ All tests completed!');
        
    } catch (error) {
        console.error('❌ Test failed:', error);
    } finally {
        if (ws) {
            ws.close();
        }
    }
}

// Run tests
runTests();
