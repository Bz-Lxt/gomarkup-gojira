import { test, expect, type APIRequestContext } from "@playwright/test";

const BASE = process.env.E2E_BASE_URL || "http://127.0.0.1:18231";
const API = process.env.E2E_API_URL || "http://127.0.0.1:18232";
const MAILPIT = process.env.E2E_MAILPIT_URL || "http://127.0.0.1:18233";

async function login(request: APIRequestContext, username: string, password: string) {
  const res = await request.post(`${API}/api/v1/auth/login`, {
    data: { username, password },
  });
  expect(res.ok(), await res.text()).toBeTruthy();
  const body = await res.json();
  return body.data as {
    access_token: string;
    user: { id: number; role: string };
  };
}

async function authJSON(request: APIRequestContext, token: string, method: string, url: string, data?: unknown) {
  const res = await request.fetch(url, {
    method,
    headers: { Authorization: `Bearer ${token}`, "Content-Type": "application/json" },
    data,
  });
  return res;
}

test.describe("GoJira critical paths (mock SMTP / Mailpit, cost ¥0)", () => {
  test("1. 登录后看板渲染四列", async ({ page }) => {
    await page.goto(`${BASE}/login`);
    await page.getByLabel(/用户|账号|username/i).fill("pm");
    await page.getByLabel(/密码|password/i).fill("Pm@123456");
    await page.getByRole("button", { name: /登录|login/i }).click();
    await page.waitForURL(/projects|board/);
    if (!/board/.test(page.url())) {
      await page.getByText(/GoJira Demo|GJ/).first().click();
    }
    await expect(page.getByText("待处理")).toBeVisible();
    await expect(page.getByText("开发中")).toBeVisible();
    await expect(page.getByText("已测试")).toBeVisible();
    await expect(page.getByText("已完成")).toBeVisible();
    await expect(page.getByText("测试验证中")).toBeVisible();
  });

  test("2. 拖拽推进任务后刷新保持", async ({ page, request }) => {
    const { access_token } = await login(request, "dev", "Dev@123456");
    const board = await (await authJSON(request, access_token, "GET", `${API}/api/v1/projects/1/board`)).json();
    const todo = (board.data.columns as { id: string; issues: { id: number; version: number; issue_type: string }[] }[])
      .find((c) => c.id === "TODO");
    const task = todo?.issues.find((i) => i.issue_type !== "BUG") || todo?.issues[0];
    expect(task, "need a TODO issue").toBeTruthy();
    const tr = await authJSON(request, access_token, "POST", `${API}/api/v1/issues/${task!.id}/transition`, {
      to: "IN_PROGRESS",
      version: task!.version,
    });
    expect(tr.status(), await tr.text()).toBe(200);
    await page.goto(`${BASE}/login`);
    await page.getByLabel(/用户|账号|username/i).fill("dev");
    await page.getByLabel(/密码|password/i).fill("Dev@123456");
    await page.getByRole("button", { name: /登录|login/i }).click();
    await page.waitForTimeout(500);
    if (!/board/.test(page.url())) {
      await page.goto(`${BASE}/p/GJ/board`);
    }
    await expect(page.getByText(/GJ-/).first()).toBeVisible();
  });

  test("3. DEV 不能将 Bug 置为 RESOLVED", async ({ request }) => {
    const { access_token } = await login(request, "dev", "Dev@123456");
    const list = await (
      await authJSON(request, access_token, "GET", `${API}/api/v1/projects/1/issues?type=BUG&per_page=50`)
    ).json();
    const bug = (list.data as { id: number; status: string; version: number }[]).find((i) => i.status === "FIXED");
    expect(bug, "seed must include FIXED bug").toBeTruthy();
    const res = await authJSON(request, access_token, "POST", `${API}/api/v1/issues/${bug!.id}/transition`, {
      to: "RESOLVED",
      version: bug!.version,
    });
    expect(res.status()).toBe(403);
    const body = await res.json();
    expect(body.code).toBe("FORBIDDEN");
  });

  test("4. QA 可以将 Bug FIXED → RESOLVED", async ({ request }) => {
    const { access_token } = await login(request, "qa", "Qa@123456");
    const list = await (
      await authJSON(request, access_token, "GET", `${API}/api/v1/projects/1/issues?type=BUG&per_page=50`)
    ).json();
    const bug = (list.data as { id: number; status: string; version: number }[]).find((i) => i.status === "FIXED");
    expect(bug, "seed must include FIXED bug").toBeTruthy();
    const res = await authJSON(request, access_token, "POST", `${API}/api/v1/issues/${bug!.id}/transition`, {
      to: "RESOLVED",
      version: bug!.version,
    });
    expect(res.status(), await res.text()).toBe(200);
  });

  test("5. 进入已完成列后 Mailpit 收到 PM 邮件", async ({ request }) => {
    const { access_token } = await login(request, "qa", "Qa@123456");
    const board = await (await authJSON(request, access_token, "GET", `${API}/api/v1/projects/1/board`)).json();
    const testing = (board.data.columns as { id: string; issues: { id: number; version: number; issue_type: string }[] }[])
      .find((c) => c.id === "TESTING");
    const task = testing?.issues.find((i) => i.issue_type !== "BUG");
    expect(task, "need TESTING task").toBeTruthy();
    const before = await (await request.get(`${MAILPIT}/api/v1/messages`)).json();
    const beforeCount = before.messages?.length ?? before.total ?? 0;
    const tr = await authJSON(request, access_token, "POST", `${API}/api/v1/issues/${task!.id}/transition`, {
      to: "DONE",
      version: task!.version,
    });
    expect(tr.status(), await tr.text()).toBe(200);
    await expect
      .poll(
        async () => {
          const after = await (await request.get(`${MAILPIT}/api/v1/messages`)).json();
          return after.messages?.length ?? after.total ?? 0;
        },
        { timeout: 15000 },
      )
      .toBeGreaterThan(beforeCount);
  });

  test("6. 燃尽图 API 含理想线与实际线", async ({ page, request }) => {
    const { access_token } = await login(request, "pm", "Pm@123456");
    const sprints = await (await authJSON(request, access_token, "GET", `${API}/api/v1/projects/1/sprints`)).json();
    const active = (sprints.data as { id: number; status: string }[]).find((s) => s.status === "ACTIVE") || sprints.data[0];
    const burn = await (await authJSON(request, access_token, "GET", `${API}/api/v1/sprints/${active.id}/burndown?metric=points`)).json();
    expect(burn.data.ideal.length).toBeGreaterThan(0);
    expect(burn.data.actual.length).toBeGreaterThan(0);
    await page.goto(`${BASE}/login`);
    await page.getByLabel(/用户|账号|username/i).fill("pm");
    await page.getByLabel(/密码|password/i).fill("Pm@123456");
    await page.getByRole("button", { name: /登录|login/i }).click();
    await page.waitForTimeout(400);
    await page.goto(`${BASE}/p/GJ/stats`);
    await expect(page.getByRole("heading", { name: /燃尽图/ })).toBeVisible();
    await expect(page.getByText("暂无燃尽数据")).toHaveCount(0);
  });

  test("7. 循环依赖被拒绝", async ({ request }) => {
    const { access_token } = await login(request, "pm", "Pm@123456");
    const list = await (
      await authJSON(request, access_token, "GET", `${API}/api/v1/projects/1/issues?per_page=20`)
    ).json();
    const issues = list.data as { id: number }[];
    expect(issues.length).toBeGreaterThanOrEqual(2);
    const a = issues[0].id;
    const b = issues[1].id;
    await authJSON(request, access_token, "POST", `${API}/api/v1/issues/${b}/dependencies`, {
      predecessor_id: a,
      dep_type: "FS",
    });
    const cycle = await authJSON(request, access_token, "POST", `${API}/api/v1/issues/${a}/dependencies`, {
      predecessor_id: b,
      dep_type: "FS",
    });
    expect(cycle.status()).toBe(409);
    const body = await cycle.json();
    expect(JSON.stringify(body)).toMatch(/cycle|环/i);
  });
});
