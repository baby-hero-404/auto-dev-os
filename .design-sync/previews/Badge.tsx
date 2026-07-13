import { Badge } from "../../web/src/components/ui/badge";

export function Difficulty() {
  return (
    <div style={{ display: "flex", gap: 8 }}>
      <Badge value="easy" />
      <Badge value="medium" />
      <Badge value="hard" />
    </div>
  );
}

export function TaskStatus() {
  return (
    <div style={{ display: "flex", flexWrap: "wrap", gap: 8 }}>
      <Badge value="todo" />
      <Badge value="analyzing" />
      <Badge value="coding" />
      <Badge value="human_review" />
      <Badge value="merged" />
    </div>
  );
}

export function ReviewOutcome() {
  return (
    <div style={{ display: "flex", flexWrap: "wrap", gap: 8 }}>
      <Badge value="pending_review" />
      <Badge value="changes_requested" />
      <Badge value="approved" />
      <Badge value="auto_approved" />
    </div>
  );
}
