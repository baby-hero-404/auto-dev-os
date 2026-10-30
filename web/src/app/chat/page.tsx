"use client";

import { createClient } from "@aicli/ui/client";
import { AicliProvider, Conversation, ConversationList } from "@aicli/ui/react";
import { useMemo, useState } from "react";
import { DashboardLayout } from "@/components/dashboard/dashboard-layout";
import { API_BASE } from "@/lib/api/client";
import { useSession } from "@/lib/session";
import { useAuthStore } from "@/lib/store/use-auth-store";
import "./chat.css";

// Chat with the AI CLI of the organization's stored credential (admins only; the server answers
// 403 to anyone else and the list shows that message).
export default function ChatPage() {
  const session = useSession();
  const orgID = session?.user.org_id ?? "";
  const [selected, setSelected] = useState<string | undefined>();

  const client = useMemo(
    () =>
      createClient({
        baseUrl: `${API_BASE}/organizations/${orgID}/chat`,
        // read per request: the token is refreshed by the rest of the app
        getHeaders: (): Record<string, string> => {
          const token = useAuthStore.getState().session?.token;
          return token ? { Authorization: `Bearer ${token}` } : {};
        },
      }),
    [orgID],
  );

  return (
    <DashboardLayout>
      <h1 className="mb-4 text-xl font-semibold">Chat with the AI CLI</h1>
      {orgID ? (
        <AicliProvider client={client}>
          <div className="chat-layout">
            <ConversationList profile="antigravity_notools" {...(selected ? { selectedId: selected } : {})} onSelect={setSelected} />
            {selected ? <Conversation conversationId={selected} /> : <p className="aicli-empty">Pick a conversation or start a new one.</p>}
          </div>
        </AicliProvider>
      ) : null}
    </DashboardLayout>
  );
}
