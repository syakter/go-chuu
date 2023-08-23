#!/Users/syakter/Projects/go-chuu/.venv/bin/python
import requests
import json
import shutil
import os
import tempfile
import glob
import argparse
from PIL import Image

API_URL = 'http://ws.audioscrobbler.com/2.0?method='
API_KEY = '13461b7f6321e69d28b9ee51ac8521f5'

def get_weekly_album_mbids(username: str):
    r = requests.get(API_URL + 'user.getweeklyalbumchart&user=' + username + '&api_key=' + API_KEY + '&format=json')
    return json.loads(r.text)['weeklyalbumchart']['album']

def query_mbid(username):
    albums = get_weekly_album_mbids(username)
    mbids = []
    for i in range(10):
        album = albums[i]
        mbid = album['mbid']
        mbids.append(mbid)
    
    return mbids

def get_album_images(username):
    mbids = query_mbid(username)
    image_urls = []
    for m in mbids:
        r = requests.get(API_URL + 'album.getinfo&api_key=' + API_KEY + '&mbid=' + m + '&format=json')
        r = json.loads(r.text)
        if 'album' in r:
            image = r['album']['image'][-1]['#text']
            image_urls.append(image)
    
    return image_urls

def get_weekly_album_arts(username):
    img_urls = get_album_images(username)
    filenames = []
    for url in img_urls:
        if not url:
            continue
        res = requests.get(url, stream=True)
        filename = url.split('/')[-1]
        filenames.append(filename)
        if res.status_code == 200:
            with open(filename, 'wb') as f:
                shutil.copyfileobj(res.raw, f)
            print(f"Successfully downloaded image: {filename}")
        else:
            print(f"Could not retrieve image: {filename}")
    return filenames

def make_weekly_collage(username):
    collage = Image.new("RGBA", (900, 900))
    old_dir = os.getcwd()
    with tempfile.TemporaryDirectory() as tmpdir:
        os.chdir(tmpdir)
        filenames = get_weekly_album_arts(username)
        files = []
        for f in filenames:
            img = Image.open(f)
            files.append(img)
        for i in range(0, 900, 300):
            for j in range(0, 900, 300):
                if not files:
                    break
                collage.paste(files.pop(), (j, i))
            if not files:
                break
        os.chdir(old_dir)
    collage.save('collage.png')
    return 'collage'


def main():
    parser = argparse.ArgumentParser(
        prog='collage.py',
        description='creates last.fm weekly top album collage',
    )
    parser.add_argument('-u', '--username', type=str, action='store', required=True)
    args = parser.parse_args()
    collage_name = make_weekly_collage(args.username)
    return collage_name

if __name__ == '__main__':
    main()