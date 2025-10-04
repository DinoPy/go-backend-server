const WebSocket = require('ws');

function testTaskCreation() {
  return new Promise((resolve) => {
    console.log('\n🧪 Testing: Task Creation');
    
    const ws = new WebSocket('ws://localhost:8080/ws/taskbar');
    
    ws.on('open', function open() {
      console.log('🔌 Connected to WebSocket');
      ws.send(JSON.stringify({
        event: 'connect',
        data: {
          id: '28c07fc5-2732-47c0-b305-92982fbddcef',
          email: 'test@example.com',
          first_name: 'Test',
          last_name: 'User',
          google_uid: '1234567890abcdef'
        }
      }));
    });

    ws.on('message', function message(data) {
      try {
        const msg = JSON.parse(data.toString());
        console.log('📨 Received:', msg.event, msg.data ? (msg.data.id || msg.data.title || msg.data.type || msg.data.message) : '');
        
        if (msg.event === 'connected') {
          console.log('✅ User connected successfully');
          console.log('📝 Creating test task...');
          ws.send(JSON.stringify({
            event: 'task_create',
            data: {
              title: 'Test Task',
              description: 'Test description',
              duration: '01:00:00',
              category: 'test',
              tags: ['test'],
              priority: 1,
              due_at: null,
              show_before_due_time: null
            }
          }));
        } else if (msg.event === 'new_task_created') {
          console.log('✅ Task created successfully:', msg.data.id);
          ws.close();
        } else if (msg.event === 'connection_error') {
          console.log('❌ Error:', msg.data.type, '-', msg.data.message);
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
      resolve();
    });

    // Close after 5 seconds
    setTimeout(() => {
      if (ws.readyState === WebSocket.OPEN) {
        ws.close();
      }
    }, 5000);
  });
}

testTaskCreation().then(() => {
  console.log('\n🏁 Task creation test completed');
  process.exit(0);
});
