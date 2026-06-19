UPDATE tasks
SET status = 'assigned'
WHERE status = 'todo';

UPDATE tasks
SET status = 'planning'
WHERE status = 'analyzing';

UPDATE tasks
SET status = 'in_progress'
WHERE status = 'coding';

UPDATE tasks
SET status = 'completed'
WHERE status = 'merged';
