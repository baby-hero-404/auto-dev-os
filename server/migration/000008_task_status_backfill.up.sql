UPDATE tasks
SET status = 'todo'
WHERE status = 'assigned';

UPDATE tasks
SET status = 'analyzing'
WHERE status = 'planning';

UPDATE tasks
SET status = 'coding'
WHERE status = 'in_progress';

UPDATE tasks
SET status = 'merged'
WHERE status = 'completed';
