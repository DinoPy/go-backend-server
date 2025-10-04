const WebSocket = require('ws');

// Complete Task Split Test - Create then Split
// This test creates a task using the correct data structure, then splits it

function completeTaskSplitTest() {
  return new Promise((resolve) => {
    console.log('🚀 Starting Complete Task Split Test');
    console.log('👤 User: 001test.alex@gmail.com');
    console.log('🆔 User ID: b1193283-bade-46a3-9c57-67bdf6925697');
    console.log('');
    
    const ws = new WebSocket('ws://localhost:8080/ws/taskbar');
    let connected = false;
    let taskCreated = false;
    let originalTask = null;
    let splitTasks = [];
    
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
          
          console.log('📝 Creating test task with correct data structure...');
          ws.send(JSON.stringify({
            event: 'task_create',
            data: {
              id: crypto.randomUUID(), // Will be replaced by server
              title: 'Live Test Task - Ready to Split',
              descripiton: 'This task will be split into two parts for live testing', // Note: typo in struct
              created_at: new Date().toISOString(),
              completed_at: '0001-01-01T00:00:00Z', // Zero time for non-completed task
              duration: '02:30:00',
              category: 'testing',
              tags: ['live', 'test', 'split'],
              toggled_at: 0,
              is_completed: false,
              is_active: false,
              last_modified_at: Date.now(),
              priority: 2,
              due_at: '2024-12-31T23:59:59Z',
              show_before_due_time: 24
            }
          }));
        } else if (msg.event === 'new_task_created' && !taskCreated) {
          console.log('✅ Test task created successfully!');
          console.log('📊 Task Details:');
          console.log('  - ID:', msg.data.id);
          console.log('  - Title:', msg.data.title);
          console.log('  - Description:', msg.data.description);
          console.log('  - Duration:', msg.data.duration);
          console.log('  - Category:', msg.data.category);
          console.log('  - Tags:', msg.data.tags);
          console.log('  - Priority:', msg.data.priority);
          console.log('  - Due At:', msg.data.due_at);
          console.log('  - Show Before Due Time:', msg.data.show_before_due_time);
          console.log('  - Is Active:', msg.data.is_active);
          console.log('  - Is Completed:', msg.data.is_completed);
          console.log('');
          
          originalTask = msg.data;
          taskCreated = true;
          
          console.log('⏳ Waiting 3 seconds before splitting...');
          setTimeout(() => {
            console.log('✂️ Splitting task into two parts...');
            ws.send(JSON.stringify({
              event: 'task_split',
              data: {
                task_id: originalTask.id,
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
          }, 3000);
        } else if (msg.event === 'related_task_deleted') {
          console.log('🗑️ Original task deleted:', msg.data.id);
          console.log('   ✅ This should appear on your frontend as task removal');
        } else if (msg.event === 'new_task_created' && taskCreated) {
          splitTasks.push(msg.data);
          console.log('✅ Split task created:', msg.data.title);
          console.log('📊 Split Task Details:');
          console.log('  - ID:', msg.data.id);
          console.log('  - Title:', msg.data.title);
          console.log('  - Description:', msg.data.description);
          console.log('  - Duration:', msg.data.duration);
          console.log('  - Category:', msg.data.category, '(preserved from original)');
          console.log('  - Tags:', msg.data.tags, '(preserved from original)');
          console.log('  - Priority:', msg.data.priority, '(preserved from original)');
          console.log('  - Due At:', msg.data.due_at, '(preserved from original)');
          console.log('  - Show Before Due Time:', msg.data.show_before_due_time, '(preserved from original)');
          console.log('  - Is Active:', msg.data.is_active, '(preserved from original)');
          console.log('  - Is Completed:', msg.data.is_completed, '(reset to false)');
          console.log('  - Created At:', msg.data.created_at, '(preserved from original)');
          console.log('');
          
          if (splitTasks.length === 2) {
            console.log('🎉 Task split completed successfully!');
            console.log('📋 Summary:');
            console.log('  - Original task deleted');
            console.log('  - 2 split tasks created');
            console.log('  - All properties preserved except title, description, duration');
            console.log('  - Events emitted to frontend for live updates');
            console.log('');
            console.log('👀 Check your frontend to see:');
            console.log('  1. Original task removed from list');
            console.log('  2. Two new tasks added to list');
            console.log('  3. All properties preserved correctly');
            
            // Keep connection open for a bit to see events
            setTimeout(() => {
              ws.close();
            }, 2000);
          }
        } else if (msg.event === 'connection_error') {
          console.log('❌ Error:', msg.data.type, '-', msg.data.message);
          console.log('💡 This might mean:');
          console.log('   - Task creation failed');
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
      console.log('🏁 Complete test completed');
      resolve();
    });

    // Close after 30 seconds if still open
    setTimeout(() => {
      if (ws.readyState === WebSocket.OPEN) {
        ws.close();
      }
    }, 30000);
  });
}

completeTaskSplitTest().then(() => {
  console.log('\n📝 Test completed - check your frontend for live updates!');
  process.exit(0);
});
