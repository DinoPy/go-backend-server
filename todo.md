[] Fix task complete/delete slider, it allows for vertical scroll while horisontally sliding breaking the functionality.
[x] Currently the user ID is hard coded we need some functionality on connect so if the user does not exist it will be created.
[x] Add a field for the unique ID returned from google oAuth and use it as a verification method for the user, if the ID from the frontend doesn't match the ID we have on the server for that email, we will reject the "authentication". 
[] Order Add or Edit category lists by usage, we can either count each category for all tasks but this may quickly slow the query since it is n based. Or we can move the categories to their own table and have more properties attached to them such as count that will increment for each new task that's created using that particular category.
[] Maybe add a notification icon on the task bar that will receive all notifications the phone would normally receive.


### Better filtering for completed tasks
[] Tags auto fill - the tags will become a db item, will hold a count of which is more often used and we will order DESC based on that, we will also have a search that will return these tags.
[] There will be more filters such as select what tags to be included, which to be excluded. 
[] Make categories be multi select
[] Add another set of filters for all existent filters, these will allow the exclusion of the selection.
[] + Other filters.


### Task-lists ← feature that will break a task into sub-tasks. It will have times for each sub task and once played the UI for the subtask will show each subtask counting down. For example an app for workout. It will have warm-up 5 minutes (could be broken down) → Set weights 20 sec → Exercise 1 min → break 1.5 min → change weight 20 sec → exercise 1 min and so on.
[] Needs some thinking to see how it will combine with the rest of the apps.
[] The app should allow grouping and that group to repeat a given number of times.
[] But it should also allow not grouping, the idea being that one task such as “set-up laptop” can be broken down into “reset, install, update, set-up user” etc.
[] It should allow live edit, just the same way the current tasks are added and edited, the task list should allow to actively add/delete tasks to it, toggle them, not complete though, they will complete in bulk along with the parent. 
[] Each sub-task will be exactly the same as a task, it will allow for all features of task but it will be grouped within the task-list of another parent task.
[] [ALTERNATIVE] The event can be related to a task, and the set of items and their times can go in the description of the event below a text description if applicable. Maybe each item can have its own description 
[] The task-list as thought for the gym example should allow for a pause on tasks, or a pause task that's toggled when no other sub-task is toggled. The idea being that while exercising maybe you need to leave a note.
[] The items in the task-list should count from 0 to the optional estimated time, and when going past it would just go on but maybe change the UI visually. Maybe also add a property in DB where the task is marked as past estimated time.
[] Given the parent task and sub-task list the completed screen should change as well.
    [] It should show all tasks and subtasks below but indented, maybe within the same parent container with the parent task so they seem related.
        [] When searching for a task by name it should find the sub-task, and the parent and return both and the parent can have a button somewhere to show the rest of subtasks within that task list.


### Alarm [Only for mobile] - ideally the alarm will stop based on one or more physical actions. Be it scanning a QR or doing some physical exercise.
[] It would be best if the alarm, after being toggled of it will be connected to a task or a task list and you'd have to start doing these tasks. 
    [] Maybe only allow the alarm to be disabled once all the sub-tasks in the task list or the task itself are completed.  But this would require a method to ensure that the actions are actually done and not completed ahead of time.
        [] Possibly, make a task list with exagerated estimated times and unless you complete the task before the time expires it would start ringing again and start the same task from 0, the one that was not manually closed.
    [] Maybe check for inactivity with the phone, movement, inactivity (while the phone is forced awake).


### Reminder feature - it should allow reminders that are recurring at certain dates, times, hours, patterns of times. 
[] The reminder will stay on the backend and will be sent to the fronde d applications until acknowledged. It will be resent every some time. It will also be sent as a notification the the mobile app
[] They should take custom sounds per reminder defaulted to something. 
[] The reminder will first sing, maybe until closed, maybe only make a notification sound and repeat until acknowledged.
    [] Should allow postponing and changing of the reminder date.
[] The reminders should become a task once acknowledged.
[] _Maybe when creating a reminder that's linked to a list, possibly only if the reminder is recurring give some custom fields that will be used for some stats. Such as along with a list of things to do in the morning when waking up, the list is connected to an alarm but is still a list and to this list we can add sleep parameters taken from the watch + a score to a set of custom parameters._


### Schedule task creation - similar to asana, schedule tasks to be created based on a pattern. A repeat feature of a task that already exists.
[] This needs to be well thought to see how it would combine with the rest of the task features.
[] Create a feature that will schedule task creation based on templates maybe? Must think of this one


### Todo list 
[] Maybe with a way to set reminders to complete the to do list or do the list. Maybe with templates for certain situations such as morning action list (a list of items to do after waking up to stay awake and transition to wakefullness), or a list of items to take when going out somewhere. A default list. 


### Templates
[] Create a duplicate of the tasks table and use it as templates. 
[] We can have a save as template button in the add task.
[] Also in add task we can have a button that will show templates.
    [] When selecting at template for a new task it should populate the add task view with the properties matching the template.
[] Templates need to be edited so there will be an edit button somewhere.


### Widget
[] We need a widget that will be present all the time at the top of the notification box to show the current task.


### Non active task reaction
[] When no task is active something must happen with the background in the app so I don't forget.


### Due date
[] Add properties to tasks with due date and defaulted value for how long before due to show the task to the user.
[] The tasks are only shown if within the show_task_within time.
[] There will be a button that will toggle showing all tasks or will show a pop-up with a list of all tasks that are to be created.


### Sorting - the sorting should happen in the local app not the server, the server willl have a fixed pattern that it will respect when returning task lists, and that list will be adjusted on the local app. The default will be by creation date, which is what the server will be returning and upon context menu selection it should filter by priority level or category. Maybe have a shortcut for this as well, maybe have an input box similar to control shift S that will take letters which will result in triggering a command. The objective being not to conflict or bloat the global shortcut commands.
[] By Priority
    [] Add a priority level. A number from 1 to 10 or even more, that can be null when a task is created it won't have a priority level, and without a priority level it will be filtered last after the higher priority tasks.
[] By category
[] By creation date
[] By distance to due date


### Edit task shortcut
[] Add a shortcut to trigger the edit for a task similar to control + shift + s. Command → Input box → Select task by some identifier, likely if visible we will use the index, and open an edit menu auto populated for that task.


### Task visibility and ordering [desktop app]
[] Add a feature to multi-select what categories to see at one time + 2 numbers of all tasks and current tasks shown.
[] If all tasks from a category are closed, then default back to all.


### Themes on desktop app, something neutral that that would make the tasks more readable.


### Task grouping
[] Primary task will include multiple tasks, there should be a relation between the primary task and the ones that are included. The idea being to have a parent task that has multiple smaller tasks and the parent task duration will be based on the summed value of all tasks.
[] This will make it easier to create a task that has components rather than add bullet list in the description to break down what happened in that vague named task.


### Additional tasks functionality
[] Split - the split tasks will all have the creation date of the original task only the duration is the value that's split. 
[] Duplicate



[] Review the authentication logic and revamp the method so it can be more secure.