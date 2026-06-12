import argparse
import logging
import os

from huggingface_hub import HfApi, hf_hub_download

logger = logging.getLogger(__name__)

REPO_ID = "VisioLab/CS2-Cheating-Detection"
REPO_TYPE = "dataset"


def download_cs2cd(output_dir: str, max_matches: int = 30):
    api = HfApi()
    all_files = api.list_repo_files(repo_id=REPO_ID, repo_type=REPO_TYPE)

    cheater_csvs = sorted(
        f for f in all_files
        if "with_cheater_present" in f and f.endswith(".csv.gz")
    )
    clean_csvs = sorted(
        f for f in all_files
        if "no_cheater_present" in f and f.endswith(".csv.gz")
    )

    half = max_matches // 2
    selected_cheater = cheater_csvs[:half]
    selected_clean = clean_csvs[:half]

    selected_files = []
    for csv_path in selected_cheater + selected_clean:
        json_path = csv_path.replace(".csv.gz", ".json")
        selected_files.append(csv_path)
        if json_path in all_files:
            selected_files.append(json_path)

    total = len(selected_files)
    logger.info("Downloading %d files (%d matches) to %s", total, max_matches, output_dir)

    for file_index, file_path in enumerate(selected_files):
        local_path = os.path.join(output_dir, file_path)
        if os.path.exists(local_path):
            logger.info("[%d/%d] Already exists: %s", file_index + 1, total, file_path)
            continue
        os.makedirs(os.path.dirname(local_path), exist_ok=True)
        logger.info("[%d/%d] Downloading: %s", file_index + 1, total, file_path)
        downloaded = hf_hub_download(
            repo_id=REPO_ID,
            repo_type=REPO_TYPE,
            filename=file_path,
            local_dir=output_dir,
        )
        logger.info("  Saved to: %s", downloaded)

    logger.info("Download complete: %d files", total)


if __name__ == "__main__":
    logging.basicConfig(level=logging.INFO, format="%(asctime)s %(levelname)s %(message)s")
    parser = argparse.ArgumentParser(description="Download CS2CD dataset subset")
    parser.add_argument("--output", default="datasets/cs2cd", help="Output directory")
    parser.add_argument("--matches", type=int, default=30, help="Number of matches to download")
    parsed_args = parser.parse_args()
    download_cs2cd(parsed_args.output, parsed_args.matches)
