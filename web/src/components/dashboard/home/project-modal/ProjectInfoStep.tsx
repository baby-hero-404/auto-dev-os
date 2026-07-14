"use client";

import { FormEvent } from "react";
import { ArrowLeft } from "lucide-react";
import { Input } from "@/components/ui/input";
import { Textarea } from "@/components/ui/textarea";
import { Field } from "@/components/ui/field";
import { Button } from "@/components/ui/button";

interface ProjectInfoStepProps {
  projectName: string;
  setProjectName: (val: string) => void;
  projectDescription: string;
  setProjectDescription: (val: string) => void;
  isSubmitting: boolean;
  creationError: string;
  onNext: (e: FormEvent) => void;
  onCancel: () => void;
}

export function ProjectInfoStep({
  projectName,
  setProjectName,
  projectDescription,
  setProjectDescription,
  isSubmitting,
  creationError,
  onNext,
  onCancel,
}: ProjectInfoStepProps) {
  return (
    <form className="mt-2 flex flex-col gap-4" onSubmit={onNext}>
      <Field label="Name" htmlFor="project-name" error={creationError}>
        <Input
          id="project-name"
          value={projectName}
          onChange={(e) => setProjectName(e.target.value)}
          placeholder="e.g. api-backend"
          disabled={isSubmitting}
          required
          autoFocus
        />
      </Field>

      <Field label="Description" htmlFor="project-desc">
        <Textarea
          id="project-desc"
          value={projectDescription}
          onChange={(e) => setProjectDescription(e.target.value)}
          placeholder="Optional goal, scope, or repository context."
          disabled={isSubmitting}
          className="resize-none"
        />
      </Field>

      <div className="mt-2 flex items-center justify-end gap-3 border-t border-stroke pt-4">
        <Button
          variant="secondary"
          onClick={onCancel}
          disabled={isSubmitting}
          type="button"
          size="sm"
        >
          Cancel
        </Button>
        <Button
          variant="primary"
          disabled={isSubmitting}
          type="submit"
          size="sm"
        >
          Next
          <ArrowLeft size={16} className="rotate-180" />
        </Button>
      </div>
    </form>
  );
}
