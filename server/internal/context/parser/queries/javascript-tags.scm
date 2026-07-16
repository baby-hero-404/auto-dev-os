; Adapted from references/aider/aider/queries (aider/repomap.py's own tag source, this project's
; stated inspiration — see docs/features/engineering/01-context-management.md), dropping Aider's
; doc-comment-association predicates (#strip!/#select-adjacent! — Aider implements those itself
; in repomap.py; nothing here consumes a @doc capture, so they'd be dead weight) and verified
; against this grammar directly rather than assumed compatible with Aider's Python-side grammar
; version. Shared by .js/.jsx/.mjs/.cjs.
;
; variable_declarator/assignment_expression/pair+arrow_function all matter because plain
; `function foo() {}` is only one of several ways JS/React code defines a named function —
; `const handler = () => {}`, `this.handler = () => {}`, and `{ method() {} }` object-literal
; shorthand are at least as common (this codebase's own web/ frontend leans on the first two
; throughout), and a query that only matched function_declaration would silently treat all of
; them as if they didn't exist.

(function_declaration name: (identifier) @name.definition.function) @definition.function
(function_expression name: (identifier) @name.definition.function) @definition.function
(generator_function_declaration name: (identifier) @name.definition.function) @definition.function
(class_declaration name: (identifier) @name.definition.class) @definition.class
(method_definition name: (property_identifier) @name.definition.method) @definition.method
(variable_declarator name: (identifier) @name.definition.function value: [(arrow_function) (function_expression)]) @definition.function
(assignment_expression left: [(identifier) @name.definition.function (member_expression property: (property_identifier) @name.definition.function)] right: [(arrow_function) (function_expression)]) @definition.function
(pair key: (property_identifier) @name.definition.function value: [(arrow_function) (function_expression)]) @definition.function
(call_expression function: (identifier) @name.reference.call)
(call_expression function: (member_expression property: (property_identifier) @name.reference.call))
(new_expression constructor: (identifier) @name.reference.call)
