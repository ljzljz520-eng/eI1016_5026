const statusGrid = document.querySelector("#status-grid");
const auditBody = document.querySelector("#audit-body");

const systemCodes = {
  elevator: "EL",
  hvac: "AC",
  access: "DR"
};

function element(tag, className, text) {
  const node = document.createElement(tag);
  if (className) node.className = className;
  if (text !== undefined) node.textContent = text;
  return node;
}

function renderSystems(assets) {
  statusGrid.replaceChildren();
  let online = 0;
  let tickets = 0;
  for (const asset of assets) {
    if (asset.state === "operational") online += 1;
    tickets += asset.openTickets;
    const card = element("article", "status-card");
    const header = element("div", "status-card-header");
    header.append(element("span", `system-code ${asset.kind}`, systemCodes[asset.kind] || "SY"));
    header.append(element("span", `state-badge ${asset.state}`, asset.state));
    const title = element("h3", "", asset.name);
    const detail = element("p", "status-detail", asset.detail);
    const footer = element("div", "status-meta");
    footer.append(element("span", "", `Updated ${asset.updatedAt}`));
    footer.append(element("strong", "", `${asset.openTickets} tickets`));
    card.append(header, title, detail, footer);
    statusGrid.append(card);
  }
  document.querySelector("#online-count").textContent = `${online}/${assets.length}`;
  document.querySelector("#ticket-count").textContent = String(tickets);
}

function renderAudit(entries) {
  auditBody.replaceChildren();
  for (const entry of entries.slice().reverse().slice(0, 8)) {
    const row = document.createElement("tr");
    const time = new Date(entry.time).toISOString().slice(11, 19);
    row.append(
      element("td", "mono", time),
      element("td", "", entry.username || "unknown"),
      element("td", "", entry.action),
      element("td", "", entry.outcome)
    );
    auditBody.append(row);
  }
  if (entries.length === 0) {
    const row = document.createElement("tr");
    const cell = element("td", "", "No activity recorded");
    cell.colSpan = 4;
    row.append(cell);
    auditBody.append(row);
  }
}

async function loadDashboard() {
  const [statusResponse, auditResponse] = await Promise.all([
    fetch("/api/status"),
    fetch("/api/audit")
  ]);
  if (statusResponse.status === 401 || auditResponse.status === 401) {
    window.location.assign("/");
    return;
  }
  if (!statusResponse.ok || !auditResponse.ok) throw new Error("dashboard request failed");
  const status = await statusResponse.json();
  const audit = await auditResponse.json();
  renderSystems(status.assets);
  renderAudit(audit.entries);
}

loadDashboard().catch(() => {
  statusGrid.replaceChildren(element("p", "error-state", "System status is temporarily unavailable."));
  auditBody.replaceChildren();
  const row = document.createElement("tr");
  const cell = element("td", "", "Audit activity is temporarily unavailable.");
  cell.colSpan = 4;
  row.append(cell);
  auditBody.append(row);
});
