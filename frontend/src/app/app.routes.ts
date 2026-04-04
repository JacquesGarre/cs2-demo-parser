import { Routes } from '@angular/router';
import { UploadPageComponent } from './features/upload/upload-page.component';
import { DashboardPageComponent } from './features/dashboard/dashboard-page.component';
import { NotFoundPageComponent } from './features/not-found/not-found-page.component';

export const routes: Routes = [
  { path: '', component: UploadPageComponent },
  { path: 'upload', redirectTo: '', pathMatch: 'full' },
  { path: 'dashboard', component: DashboardPageComponent },
  { path: '**', component: NotFoundPageComponent },
];
