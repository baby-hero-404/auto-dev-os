; MVP hardcoded query, kept minimal to avoid path resolution issues across runtime environments.
; Definitions capture both the name (@name.definition.*, used for the tag's Name) and the
; enclosing declaration (@definition.*, used for the tag's Line/EndLine span) — without the
; second capture, a definition's node is just its identifier token, so Line == EndLine always
; and callers relying on the span (function/method length, snippet extraction) silently get
; nonsense. References have no meaningful "body span" so they keep a single capture.

(function_declaration name: (identifier) @name.definition.function) @definition.function
(method_declaration name: (field_identifier) @name.definition.method) @definition.method
(type_declaration (type_spec name: (type_identifier) @name.definition.class)) @definition.class
(call_expression function: (identifier) @name.reference.call)
(call_expression function: (selector_expression field: (field_identifier) @name.reference.call))
