ALTER TABLE task_events
    ADD CONSTRAINT fk_task_events_artifact_id
    FOREIGN KEY (artifact_id) REFERENCES workflow_artifacts (id);
