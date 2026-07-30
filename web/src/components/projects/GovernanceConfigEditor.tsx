"use client";

import { useState, useEffect } from "react";
import useSWR from "swr";
import { Sliders, AlertCircle, FileJson, Info } from "lucide-react";
import { Card, CardHeader, CardContent } from "@/components/ui/card";
import { Select } from "@/components/ui/select";
import { Button } from "@/components/ui/button";
import { Field } from "@/components/ui/field";
import { governance as governanceApi } from "@/lib/api";
import type { GovernancePreset } from "@/lib/types";

interface GovernanceConfigEditorProps {
  pipelineConfig: unknown;
  token: string;
  onChange: (config: unknown) => void;
  serverError?: string;
  disabled?: boolean;
}

export function GovernanceConfigEditor({
  pipelineConfig,
  token,
  onChange,
  serverError,
  disabled = false,
}: GovernanceConfigEditorProps) {
  const [jsonText, setJsonText] = useState("");
  const [parseError, setParseError] = useState("");
  const [selectedPreset, setSelectedPreset] = useState("");

  const { data: presets } = useSWR<GovernancePreset[]>(
    token ? ["/governance/presets", token] : null,
    () => governanceApi.listPresets(token)
  );

  useEffect(() => {
    if (pipelineConfig) {
      try {
        // eslint-disable-next-line react-hooks/set-state-in-effect
        setJsonText(JSON.stringify(pipelineConfig, null, 2));
      } catch {
        // eslint-disable-next-line react-hooks/set-state-in-effect
        setJsonText("");
      }
    } else {
      // eslint-disable-next-line react-hooks/set-state-in-effect
      setJsonText("");
    }
  }, [pipelineConfig]);

  function handlePresetChange(presetName: string) {
    setSelectedPreset(presetName);
    if (!presetName) return;

    const preset = presets?.find((p) => p.name === presetName);
    if (preset) {
      const formatted = JSON.stringify(preset.config, null, 2);
      setJsonText(formatted);
      setParseError("");
      try {
        const parsed = JSON.parse(formatted);
        onChange(parsed);
      } catch {}
    }
  }

  function handleTextChange(val: string) {
    setJsonText(val);
    setSelectedPreset("");
    if (!val.trim()) {
      setParseError("");
      onChange(null);
      return;
    }

    try {
      const parsed = JSON.parse(val);
      setParseError("");
      onChange(parsed);
    } catch (err) {
      setParseError(err instanceof Error ? err.message : "Invalid JSON syntax");
    }
  }

  function handleFormat() {
    if (!jsonText.trim()) return;
    try {
      const parsed = JSON.parse(jsonText);
      setJsonText(JSON.stringify(parsed, null, 2));
      setParseError("");
    } catch (err) {
      setParseError(err instanceof Error ? err.message : "Invalid JSON syntax");
    }
  }

  return (
    <Card>
      <CardHeader
        title="Declarative Governance Pipeline Config"
        icon={<Sliders size={18} className="text-brand-primary" />}
      />
      <CardContent className="space-y-4">
        <div className="rounded-md border border-brand-primary/20 bg-brand-primary/5 p-4 text-sm text-foreground space-y-3">
          <div className="flex items-center gap-2 font-semibold text-brand-primary">
            <Info size={16} />
            <span>What is this for?</span>
          </div>
          <p className="text-content-muted leading-relaxed">
            This config controls the <strong>safety, autonomy, and review policies</strong> for AI agents in this project.
            Instead of hardcoding rules, you define them here via JSON.
          </p>
          <ul className="list-disc pl-5 text-content-muted space-y-1 text-xs">
            <li><strong>dor_gates:</strong> Conditions (e.g., pass tests) required before code can be merged.</li>
            <li><strong>review_skip:</strong> Allow the AI to skip human review for small, safe changes.</li>
            <li><strong>cycle_limits:</strong> Max number of self-correction loops the AI can run.</li>
            <li><strong>harness_policies:</strong> Restrictions on which tools or environments the AI can access.</li>
          </ul>
        </div>

        <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-3">
          <Field
            label="Load Built-in Preset"
            htmlFor="preset-picker"
            hint="Fill editor with preset pipeline template"
            className="flex-1"
          >
            <Select
              id="preset-picker"
              value={selectedPreset}
              onChange={(e) => handlePresetChange(e.target.value)}
              disabled={disabled}
            >
              <option value="">-- Custom / Select Preset --</option>
              {presets?.map((p) => (
                <option key={p.name} value={p.name}>
                  {p.name}
                </option>
              ))}
            </Select>
          </Field>

          <Button
            type="button"
            variant="secondary"
            size="sm"
            onClick={handleFormat}
            disabled={disabled || !jsonText.trim()}
            className="self-end"
          >
            <FileJson size={14} /> Format JSON
          </Button>
        </div>

        <Field
          label="Pipeline Config (JSON Schema)"
          htmlFor="governance-json"
          hint="Declarative JSON schema overriding DoR gates, review skip, cycle limits, router, and harness policies"
        >
          <textarea
            id="governance-json"
            value={jsonText}
            onChange={(e) => handleTextChange(e.target.value)}
            disabled={disabled}
            rows={10}
            placeholder={`{\n  "version": "v1",\n  "extends": "default_workflow"\n}`}
            className="w-full rounded-md border border-stroke/20 bg-background p-3 font-mono text-xs text-foreground focus:outline-none focus:ring-2 focus:ring-brand-primary disabled:opacity-50 resize-y custom-scrollbar"
          />
        </Field>

        {parseError && (
          <div className="rounded-md bg-danger/10 border border-danger/20 p-3 text-xs text-danger flex items-center gap-2">
            <AlertCircle size={14} className="shrink-0" />
            <span>JSON Syntax Error: {parseError}</span>
          </div>
        )}

        {serverError && !parseError && (
          <div className="rounded-md bg-danger/10 border border-danger/20 p-3 text-xs text-danger flex items-center gap-2">
            <AlertCircle size={14} className="shrink-0" />
            <span>Validation Error: {serverError}</span>
          </div>
        )}
      </CardContent>
    </Card>
  );
}
