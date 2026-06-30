import time
import requests
import json
import os

API_BASE_URL = os.environ.get("API_BASE_URL", "http://127.0.0.1:28001")

# 需要检索的疑似侵权词汇
TARGET_KEYWORDS = [
    "玫瑰的故事 免费全集在线观看",
    "金猪玉叶 短剧全集 百度网盘"
]

def submit_job(keyword: str) -> str:
    print(f"[*] Submitting copyright detection job for keyword: '{keyword}'")
    
    payload = {
        "job_type": "generic.browser.agent",
        "adapter": "generic.browser_agent",
        "target": {
            "url": "https://www.bing.com",
            "allowed_domains": ["bing.com", "baidu.com", "google.com", "*"]
        },
        "input": {
            "url": "https://www.bing.com",
            "mode": "llm_plan",
            "task": f"Search for '{keyword}'. Find a search result that looks like a pirated movie/streaming website or an unofficial cloud drive link. DO NOT click on official legal sites (like iqiyi, qq, bilibili). Click the unofficial link, wait for the page to load, and then take a screenshot for evidence."
        },
        "policy": {
            "allowed_actions": ["navigate", "click", "fill", "type", "screenshot", "wait", "press", "observe_page"],
            "headed": True,
            "max_duration_seconds": 600
        },
        "priority": 1
    }
    
    resp = requests.post(f"{API_BASE_URL}/admin/automation/jobs", json=payload)
    resp.raise_for_status()
    data = resp.json()
    job_id = data["job_id"]
    print(f"[+] Job created successfully. Job ID: {job_id}")
    return job_id

def poll_job_completion(job_id: str):
    print(f"[*] Polling status for job {job_id}...")
    while True:
        resp = requests.get(f"{API_BASE_URL}/admin/automation/jobs/{job_id}")
        if resp.status_code != 200:
            print(f"[-] Failed to fetch job: {resp.text}")
            time.sleep(3)
            continue
            
        job_data = resp.json()
        status = job_data.get("status")
        
        if status in ["completed", "failed", "cancelled", "expired"]:
            print(f"[*] Job finished with status: {status}")
            return job_data
            
        print(f"   ... status: {status}, waiting 5s")
        time.sleep(5)

def download_artifacts(job_id: str):
    # Get runs for the job
    print(f"[*] Fetching runs for job {job_id}...")
    resp = requests.get(f"{API_BASE_URL}/admin/automation/runs?limit=50")
    runs = resp.json().get("runs", [])
    
    # Find the run associated with this job
    run = next((r for r in runs if r["job_id"] == job_id), None)
    if not run:
        print(f"[-] Could not find run for job {job_id}")
        return
        
    run_id = run["id"]
    print(f"[*] Found Run ID: {run_id}. Fetching artifacts...")
    
    art_resp = requests.get(f"{API_BASE_URL}/admin/automation/runs/{run_id}/artifacts")
    artifacts = art_resp.json().get("artifacts", [])
    
    if not artifacts:
        print("[-] No artifacts found.")
        return
        
    # Download them
    out_dir = os.path.join("deploy-local", "run", "copyright_reports", job_id)
    os.makedirs(out_dir, exist_ok=True)
    
    report_lines = [f"# 版权侵权检测报告 - Job {job_id}\n\n"]
    
    for art in artifacts:
        art_id = art["id"]
        filename = art.get("filename", f"{art_id}.bin")
        print(f"[*] Downloading artifact: {filename}...")
        
        download_resp = requests.get(f"{API_BASE_URL}/admin/automation/artifacts/{art_id}/download")
        filepath = os.path.join(out_dir, filename)
        with open(filepath, "wb") as f:
            f.write(download_resp.content)
            
        print(f"[+] Saved {filepath}")
        
        if art.get("artifact_type") == "screenshot":
            report_lines.append(f"## 取证截图: {filename}\n")
            report_lines.append(f"![{filename}](./{filename})\n\n")
            
    report_path = os.path.join(out_dir, "report.md")
    with open(report_path, "w") as f:
        f.writelines(report_lines)
    print(f"[+] Report generated at {report_path}")

def main():
    print("=== Browser Agent: 版权侵权检测 PoC ===")
    for keyword in TARGET_KEYWORDS:
        job_id = submit_job(keyword)
        job_data = poll_job_completion(job_id)
        if job_data.get("status") == "completed":
            print("[-] Skipping artifact download (not supported in current admin API).")
        # download_artifacts(job_id)
        else:
            print(f"[-] Job failed. Cannot download artifacts.")
        print("-" * 40)
        
if __name__ == "__main__":
    main()
