import * as dotenv from 'dotenv' // see https://github.com/motdotla/dotenv#how-do-i-use-dotenv-with-import
dotenv.config()
import PocketBase from 'pocketbase'
import data from './nf.json' assert { type: "json" };

console.log("Hallo");

const pocketBaseUrl = "http://localhost:8090";

console.log(`will seed database at ${pocketBaseUrl}`);

const client = new PocketBase(pocketBaseUrl);
const user = process.env.POCKET_BASE_ADMIN_USER;
const password = process.env.POCKET_BASE_ADMIN_PASS;

if (user === undefined || password === undefined) {
  throw Error("POCKET_BASE_ADMIN_USER or POCKET_BASE_ADMIN_PASS not set");
}

const collection = client.collection("movies");

async function authenticate() {
  client.authStore.clear();
  const authData = await client.admins.authWithPassword(user, password);
  console.log(client.authStore.isValid);

  let movie_titles = [];

  for (var element of data) {
    if (!movie_titles.includes(element.title)) {
      let movie = {
        title: element.title,
        banners: element.banners,
        posters: element.posters,
        url: element.url,
        genres: element.genres,
        year: element.year,
      };
      try {
        let x = await client.collection("movies").create(movie);
        await new Promise((r) => {
          setTimeout(r, 25);
        });
        movie_titles.push(element.title);
      } catch (e) {
        console.log("verschissen bruder");
      }
    }
  }
}
console.log(`${user}:${password}`);

authenticate();
