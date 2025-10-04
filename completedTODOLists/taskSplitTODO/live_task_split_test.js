const WebSocket = require('ws');

// Manual Task Split Test
// This test allows you to manually specify a task ID to split

function manualTaskSplitTest(taskId) {
  return new Promise((resolve) => {
    console.log('🚀 Starting Manual Task Split Test');
    console.log('👤 User: 001test.alex@gmail.com');
    console.log('🆔 User ID: b1193283-bade-46a3-9c57-67bdf6925697');
    console.log('🎯 Task ID to split:', taskId);
    console.log('');
    
    const ws = new WebSocket('ws://localhost:8080/ws/taskbar');
    let connected = false;
    
    ws.on('open', function open() {
      console.log('🔌 Connected to WebSocket');
      ws.send(JSON.stringify({
        event: 'connect',
        data: {
          id: 'b1193283-bade-46a3-9c57-67bdf6925697',
          email: '001test.alex@gmail.com',
          first_name: 'Alex',
          last_name: 'Test',
          google_uid: 'test_google_uid_alex'
        }
      }));
    });

    ws.on('message', function message(data) {
      try {
        const msg = JSON.parse(data.toString());
        
        if (msg.event === 'connected') {
          console.log('✅ User connected successfully');
          connected = true;
          
          console.log('⏳ Waiting 2 seconds before splitting...');
          setTimeout(() => {
            console.log('✂️ Splitting task into two parts...');
            ws.send(JSON.stringify({
              event: 'task_split',
              data: {
                task_id: taskId,
                splits: [
                  {
                    title: 'Part 1 - Frontend Development',
                    description: 'Implement the user interface components and styling',
                    duration: '01:30:00'
                  },
                  {
                    title: 'Part 2 - Backend Integration',
                    description: 'Connect frontend to backend APIs and handle data flow',
                    duration: '01:00:00'
                  }
                ]
              }
            }));
          }, 2000);
        } else if (msg.event === 'related_task_deleted') {
          console.log('🗑️ Original task deleted:', msg.data.id);
          console.log('   ✅ This should appear on your frontend as task removal');
        } else if (msg.event === 'new_task_created') {
          console.log('✅ Split task created:', msg.data.title);
          console.log('📊 Split Task Details:');
          console.log('  - ID:', msg.data.id);
          console.log('  - Title:', msg.data.title);
          console.log('  - Description:', msg.data.description);
          console.log('  - Duration:', msg.data.duration);
          console.log('  - Category:', msg.data.category);
          console.log('  - Tags:', msg.data.tags);
          console.log('  - Priority:', msg.data.priority);
          console.log('  - Is Active:', msg.data.is_active);
          console.log('  - Is Completed:', msg.data.is_completed);
          console.log('');
          
          console.log('🎉 Task split completed successfully!');
          console.log('👀 Check your frontend to see:');
          console.log('  1. Original task removed from list');
          console.log('  2. Two new tasks added to list');
          console.log('  3. All properties preserved correctly');
          
          // Keep connection open for a bit to see events
          setTimeout(() => {
            ws.close();
          }, 3000);
        } else if (msg.event === 'connection_error') {
          console.log('❌ Error:', msg.data.type, '-', msg.data.message);
          console.log('💡 This might mean:');
          console.log('   - Task ID does not exist');
          console.log('   - Task belongs to different user');
          console.log('   - Task ID format is invalid');
          ws.close();
        } else if (msg.event === 'ping') {
          // Ignore ping messages
        } else {
          console.log('📨 Other event:', msg.event);
        }
      } catch (e) {
        console.log('📨 Raw message:', data.toString());
      }
    });

    ws.on('error', function error(err) {
      console.error('❌ WebSocket error:', err.message);
    });

    ws.on('close', function close(code, reason) {
      console.log('🔌 Connection closed:', code, reason.toString());
      console.log('🏁 Manual test completed');
      resolve();
    });

    // Close after 15 seconds if still open
    setTimeout(() => {
      if (ws.readyState === WebSocket.OPEN) {
        ws.close();
      }
    }, 15000);
  });
}

// Get task ID from command line argument
const taskId = process.argv[2];

if (!taskId) {
  console.log('❌ Please provide a task ID as an argument');
  console.log('Usage: node live_task_split_test.js <task-id>');
  console.log('');
  console.log('Example: node live_task_split_test.js 12345678-1234-1234-1234-123456789012');
  console.log('');
  console.log('💡 To get a task ID:');
  console.log('  1. Open your frontend');
  console.log('  2. Find a task you want to split');
  console.log('  3. Copy the task ID from the browser dev tools or database');
  console.log('  4. Run this script with that ID');
  process.exit(1);
}

manualTaskSplitTest(taskId).then(() => {
  console.log('\n📝 Test completed - check your frontend for live updates!');
  process.exit(0);
});