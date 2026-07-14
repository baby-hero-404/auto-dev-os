"use client";

import { useState, FormEvent } from "react";
import { Plus } from "lucide-react";
import type { Rule } from "@/lib/types";
import { EnforcementToggle } from "./EnforcementToggle";
import { ApiError } from "@/lib/api";
import { Card, CardHeader, CardContent } from "@/components/ui/card";
import { Textarea } from "@/components/ui/textarea";
import { Field } from "@/components/ui/field";
import { Button } from "@/components/ui/button";

interface AddRuleFormProps {
  onAddRule: (content: string, enforcement: Rule["enforcement"]) => Promise<Rule>;
  setNewRuleID: (id: string) => void;
  ruleError: string;
  setRuleError: (err: string) => void;
}

export function AddRuleForm({ onAddRule, setNewRuleID, ruleError, setRuleError }: AddRuleFormProps) {
  const [ruleContent, setRuleContent] = useState("");
  const [ruleEnforcement, setRuleEnforcement] = useState<Rule["enforcement"]>("strict");
  const [isAddingRule, setIsAddingRule] = useState(false);

  async function handleAddRule(e: FormEvent) {
    e.preventDefault();
    const content = ruleContent.trim();
    if (!content) {
      setRuleError("Rule content cannot be empty.");
      return;
    }
    setRuleError("");
    setIsAddingRule(true);
    try {
      const created = await onAddRule(content, ruleEnforcement);
      setRuleContent("");
      setRuleEnforcement("strict");
      setNewRuleID(created.id);
      setTimeout(() => setNewRuleID(""), 200);
    } catch (err) {
      setRuleError(err instanceof ApiError ? err.message : "Failed to add rule");
    } finally {
      setIsAddingRule(false);
    }
  }

  return (
    <Card className="h-fit">
      <CardHeader title="Add Project Rule" />
      <CardContent>
        <form onSubmit={handleAddRule} className="space-y-4">
          <Field label="Rule Guideline" htmlFor="project-rule-content">
            <Textarea
              id="project-rule-content"
              value={ruleContent}
              onChange={(e) => setRuleContent(e.target.value)}
              placeholder="e.g. Always write comprehensive unit tests using vitest for every new helper function."
              disabled={isAddingRule}
              required
              className="min-h-[100px] resize-none"
            />
          </Field>

          <EnforcementToggle value={ruleEnforcement} onChange={setRuleEnforcement} disabled={isAddingRule} />

          {ruleError && (
            <span className="text-xs text-danger font-medium leading-normal block" role="alert">
              {ruleError}
            </span>
          )}

          <Button
            type="submit"
            disabled={isAddingRule || !ruleContent.trim()}
            isLoading={isAddingRule}
            className="w-full"
          >
            {!isAddingRule && <Plus size={14} />} Add Rule
          </Button>
        </form>
      </CardContent>
    </Card>
  );
}
